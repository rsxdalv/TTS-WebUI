package react_ui_backend

// A Go port of the parts of @gradio/client 1.19.1 (dist/index.js) that
// react-ui/src/pages/api/gradio/[name].tsx uses: Client.connect, predict and
// submit over the sse_v3 queue protocol, the /run path for unqueued endpoints,
// and file uploads. Function names follow the original.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// errHang marks the cases where the Next route never answers: the client
// throws inside the async Promise executor of predict, or a rejected promise is
// never handled. The Go route waits for the browser to give up, as Next does.
var errHang = errors.New("the original never settles this request")

// errSubmitThrow is an error submit() throws synchronously. Through predict it
// hangs (it is thrown inside an async Promise executor); through a direct
// submit call it rejects.
type errSubmitThrow struct{ err error }

func (e errSubmitThrow) Error() string { return e.err.Error() }

// errReject is a rejected promise. The route answers 500.
type errReject struct{ reason any }

func (e errReject) Error() string {
	if s, ok := stringifyJSON(e.reason); ok {
		return "rejected: " + s
	}
	return "rejected"
}

const (
	brokenConnectionMsg = "Connection errored out. "
	queueFullMsg        = "This application is currently busy. Please try again. "
	configErrorMsg      = "Could not resolve app config. "
)

// gradioHTTP has no timeouts and ignores proxy settings, like Node's fetch.
var gradioHTTP = &http.Client{
	Transport: &http.Transport{
		DialContext:     (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		IdleConnTimeout: 90 * time.Second,
	},
}

type gradioClient struct {
	root        string // config.root, such as http://127.0.0.1:7770
	apiPrefix   string
	cookies     string
	config      *gradioObject
	apiInfo     *gradioObject
	apiMap      map[string]float64
	sessionHash string
	stopBeat    context.CancelFunc
}

// connectGradio is Client.connect(backend, { auth }).
func connectGradio(ctx context.Context, backend string, auth []string) (*gradioClient, error) {
	c := &gradioClient{sessionHash: newSessionHash(), apiMap: map[string]float64{}}

	ref := strings.TrimSuffix(strings.TrimSpace(backend), "/")
	u, err := url.Parse(ref)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid Gradio backend %q", backend)
	}
	host := u.Host
	if u.Path != "" && u.Path != "/" {
		host += u.Path
	}
	base := u.Scheme + "://" + host

	if auth != nil {
		if err := c.resolveCookies(ctx, base, auth); err != nil {
			return nil, err
		}
	}
	if err := c.resolveConfig(ctx, base); err != nil {
		return nil, err
	}
	c.resolveHeartbeat(ctx)
	if c.apiInfo == nil {
		info, err := c.viewAPI(ctx)
		if err != nil {
			return nil, err
		}
		c.apiInfo = info
	}
	deps, _ := c.config.get("dependencies").([]any)
	for _, d := range deps {
		dep, _ := d.(*gradioObject)
		if name, ok := dep.get("api_name").(string); ok && name != "" {
			if id, ok := dep.get("id").(float64); ok {
				c.apiMap[name] = id
			}
		}
	}
	return c, nil
}

// close stops the heartbeat stream. Next leaves it open for good; Go closes it
// when the request ends.
func (c *gradioClient) close() {
	if c.stopBeat != nil {
		c.stopBeat()
	}
}

func newSessionHash() string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 11)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		b[i] = digits[n.Int64()]
	}
	return string(b)
}

// joinURLs is join_urls.
func joinURLs(parts ...string) (string, error) {
	acc := parts[0]
	for _, part := range parts[1:] {
		base, err := url.Parse(strings.TrimRight(acc, "/") + "/")
		if err != nil {
			return "", errors.New("Invalid URL. A full URL path is required.")
		}
		ref, err := url.Parse(strings.TrimLeft(part, "/"))
		if err != nil {
			return "", errors.New("Invalid URL. A full URL path is required.")
		}
		acc = base.ResolveReference(ref).String()
	}
	return acc, nil
}

func (c *gradioClient) newRequest(ctx context.Context, method, target string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	if c.cookies != "" {
		req.Header.Set("Cookie", c.cookies)
	}
	return req, nil
}

// resolveCookies logs in with the auth pair, like resolve_cookies.
func (c *gradioClient) resolveCookies(ctx context.Context, base string, auth []string) error {
	var form bytes.Buffer
	w := multipart.NewWriter(&form)
	user, pass := "undefined", "undefined"
	if len(auth) > 0 {
		user = auth[0]
	}
	if len(auth) > 1 {
		pass = auth[1]
	}
	_ = w.WriteField("username", user)
	_ = w.WriteField("password", pass)
	_ = w.Close()

	req, err := c.newRequest(ctx, http.MethodPost, base+"/login", &form)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := gradioHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		var cookies []string
		for _, sc := range resp.Header.Values("Set-Cookie") {
			pair, _, _ := strings.Cut(sc, ";")
			name, value, _ := strings.Cut(pair, "=")
			if strings.TrimSpace(name) != "" && strings.TrimSpace(value) != "" {
				cookies = append(cookies, strings.TrimSpace(name)+"="+strings.TrimSpace(value))
			}
		}
		c.cookies = strings.Join(cookies, "; ")
		return nil
	case http.StatusUnauthorized:
		return errors.New("Invalid credentials. Could not login. ")
	}
	return errors.New("Space metadata could not be loaded. ")
}

// resolveConfig fetches /config, like resolve_config and config_success.
func (c *gradioClient) resolveConfig(ctx context.Context, base string) error {
	target, err := joinURLs(base, "config")
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := gradioHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return errors.New("Login credentials are required to access this space.")
	default:
		return errors.New(configErrorMsg)
	}
	v, err := parseJSON(body)
	if err != nil {
		return err
	}
	config, ok := v.(*gradioObject)
	if !ok {
		return errors.New(configErrorMsg)
	}
	config.set("root", base)
	if deps, ok := config.get("dependencies").([]any); ok {
		for i, d := range deps {
			if dep, ok := d.(*gradioObject); ok && dep.get("id") == undefined {
				dep.set("id", float64(i))
			}
		}
	}
	c.config = config
	c.root = base
	c.apiPrefix, _ = config.get("api_prefix").(string)
	if !isTruthy(config.get("auth_required")) {
		if info, err := c.viewAPI(ctx); err == nil {
			c.apiInfo = info
		}
	}
	return nil
}

// resolveHeartbeat opens the heartbeat stream when the app asks for one.
func (c *gradioClient) resolveHeartbeat(ctx context.Context) {
	if !isTruthy(c.config.get("connect_heartbeat")) {
		return
	}
	beatCtx, cancel := context.WithCancel(ctx)
	c.stopBeat = cancel
	target := c.root + c.apiPrefix + "/heartbeat/" + c.sessionHash
	go func() {
		req, err := c.newRequest(beatCtx, http.MethodGet, target, nil)
		if err != nil {
			return
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Content-Type", "application/json")
		resp, err := gradioHTTP.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
	}()
}

// viewAPI fetches /info and applies transform_api_info.
func (c *gradioClient) viewAPI(ctx context.Context) (*gradioObject, error) {
	fail := func(msg string) error { return errors.New("Could not get API info. " + msg) }
	target, err := joinURLs(c.root, c.apiPrefix, "info")
	if err != nil {
		return nil, fail(err.Error())
	}
	req, err := c.newRequest(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fail(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := gradioHTTP.Do(req)
	if err != nil {
		return nil, fail("fetch failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fail(brokenConnectionMsg)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fail(err.Error())
	}
	v, err := parseJSON(body)
	if err != nil {
		return nil, fail(err.Error())
	}
	info, ok := v.(*gradioObject)
	if !ok {
		return nil, fail("bad response")
	}
	if api, ok := info.get("api").(*gradioObject); ok && info.has("api") {
		info = api
	}
	named, _ := info.get("named_endpoints").(*gradioObject)
	unnamed, _ := info.get("unnamed_endpoints").(*gradioObject)
	if named == nil || unnamed == nil {
		return nil, fail("Cannot read properties of undefined")
	}
	if p := named.get("/predict"); p != undefined && unnamed.get("0") == undefined {
		unnamed.set("0", p)
	}
	return c.transformAPIInfo(info), nil
}

// transformAPIInfo inserts hidden state parameters where the endpoint's
// dependency has more inputs than the API lists, like transform_api_info.
func (c *gradioClient) transformAPIInfo(info *gradioObject) *gradioObject {
	out := obj("named_endpoints", newObject(), "unnamed_endpoints", newObject())
	deps, _ := c.config.get("dependencies").([]any)
	for _, category := range info.ownKeys() {
		if category != "named_endpoints" && category != "unnamed_endpoints" {
			continue
		}
		src, _ := info.get(category).(*gradioObject)
		dst := newObject()
		out.set(category, dst)
		if src == nil {
			continue
		}
		for _, endpoint := range src.ownKeys() {
			entry, _ := src.get(endpoint).(*gradioObject)
			params, _ := entry.get("parameters").([]any)
			params = append([]any(nil), params...)
			stripped := strings.Replace(endpoint, "/", "", 1)

			index := -1.0
			if dep := findDep(deps, func(d *gradioObject) bool {
				return d.get("api_name") == endpoint || d.get("api_name") == stripped
			}); dep != nil && isTruthy(dep.get("id")) {
				index, _ = dep.get("id").(float64)
			} else if id, ok := c.apiMap[stripped]; ok && id != 0 {
				index = id
			}
			if index != -1 {
				dep := findDep(deps, func(d *gradioObject) bool { return d.get("id") == index })
				inputs, _ := dep.get("inputs").([]any)
				if dep != nil && len(inputs) != len(params) {
					for i, input := range inputs {
						if c.componentType(input) == "state" {
							state := obj("component", "state", "example", nil, "parameter_default", nil,
								"parameter_has_default", true, "parameter_name", nil, "hidden", true)
							if i > len(params) {
								i = len(params)
							}
							params = append(params[:i], append([]any{state}, params[i:]...)...)
						}
					}
				}
			}
			dst.set(endpoint, obj("parameters", params, "returns", entry.get("returns")))
		}
	}
	return out
}

func findDep(deps []any, match func(*gradioObject) bool) *gradioObject {
	for _, d := range deps {
		if dep, ok := d.(*gradioObject); ok && match(dep) {
			return dep
		}
	}
	return nil
}

func (c *gradioClient) componentType(id any) string {
	comps, _ := c.config.get("components").([]any)
	for _, comp := range comps {
		if o, ok := comp.(*gradioObject); ok && o.get("id") == id {
			t, _ := o.get("type").(string)
			return t
		}
	}
	return ""
}

// --- submit ---

// gradioEvent is one event submit() publishes.
type gradioEvent struct {
	kind  string // "data" or "status"
	stage string // for status events
	msg   *gradioObject
}

// submit runs one call. publish receives each event and returns true to stop
// listening. submit returns closed=true when the original calls close(), which
// ends a for-await loop over the job. A nil error with closed=false means the
// original iterator would stay open.
func (c *gradioClient) submit(ctx context.Context, endpoint string, data any, publish func(gradioEvent) bool) (closed bool, err error) {
	trimmed := strings.TrimPrefix(endpoint, "/")
	fnIndex, ok := c.apiMap[trimmed]
	if !ok {
		return false, errSubmitThrow{errors.New("There is no endpoint matching that name of fn_index matching that number.")}
	}
	named, _ := c.apiInfo.get("named_endpoints").(*gradioObject)
	endpointInfo, _ := named.get(strings.TrimSpace(endpoint)).(*gradioObject)
	deps, _ := c.config.get("dependencies").([]any)
	dependency := findDep(deps, func(d *gradioObject) bool { return d.get("id") == fnIndex })

	resolved, err := mapDataToParams(data, endpointInfo)
	if err != nil {
		return false, errSubmitThrow{err}
	}
	payloadData, err := c.handleBlob(ctx, resolved, endpointInfo)
	if err != nil {
		return false, errHang
	}
	inputData := c.handlePayload(payloadData, dependency, "input", true)
	if inputData == nil {
		inputData = []any{}
	}

	stop := false
	fire := func(e gradioEvent) {
		if !stop && publish(e) {
			stop = true
		}
	}
	statusEvent := func(fields ...any) gradioEvent {
		m := obj(fields...)
		stage, _ := m.get("stage").(string)
		return gradioEvent{kind: "status", stage: stage, msg: m}
	}

	if c.skipQueue(fnIndex, dependency) {
		fire(statusEvent("type", "status", "endpoint", endpoint, "stage", "pending", "queue", false, "fn_index", fnIndex, "time", jsNow()))
		output, status := c.postData(ctx, c.root+c.apiPrefix+"/run"+ensureSlash(endpoint),
			obj("data", inputData, "event_data", nil, "fn_index", fnIndex, "trigger_id", nil, "session_hash", c.sessionHash))
		if status == 200 {
			outData, err := c.handlePayloadChecked(prop(output, "data"))(dependency)
			if err != nil {
				fire(statusEvent("type", "status", "stage", "error", "message", err.Error(), "endpoint", endpoint, "fn_index", fnIndex, "queue", false, "time", jsNow()))
				return false, nil
			}
			fire(gradioEvent{kind: "data", msg: obj("type", "data", "endpoint", endpoint, "fn_index", fnIndex,
				"data", outData, "time", jsNow(), "event_data", nil, "trigger_id", nil)})
			fire(statusEvent("type", "status", "endpoint", endpoint, "fn_index", fnIndex, "stage", "complete",
				"eta", optProp(output, "average_duration"), "queue", false, "time", jsNow()))
		} else {
			fire(statusEvent("type", "status", "stage", "error", "endpoint", endpoint, "fn_index", fnIndex,
				"message", optProp(output, "error"), "queue", false, "time", jsNow()))
		}
		return false, nil
	}

	protocol, _ := c.config.get("protocol").(string)
	switch protocol {
	case "sse_v1", "sse_v2", "sse_v2.1", "sse_v3":
	default:
		return false, fmt.Errorf("gradio protocol %q is not supported by the Go port", protocol)
	}

	fire(statusEvent("type", "status", "stage", "pending", "queue", true, "endpoint", endpoint, "fn_index", fnIndex, "time", jsNow()))
	response, status := c.postData(ctx, c.root+c.apiPrefix+"/queue/join?",
		obj("data", inputData, "event_data", nil, "fn_index", fnIndex, "trigger_id", nil, "session_hash", c.sessionHash))
	switch {
	case status == 503:
		fire(statusEvent("type", "status", "stage", "error", "message", queueFullMsg, "queue", true, "endpoint", endpoint, "fn_index", fnIndex, "time", jsNow()))
		return false, nil
	case status == 422:
		fire(statusEvent("type", "status", "stage", "error", "message", optProp(response, "detail"), "queue", true, "endpoint", endpoint, "fn_index", fnIndex, "code", "validation_error", "time", jsNow()))
		return true, nil
	case status != 200:
		fire(statusEvent("type", "status", "stage", "error", "broken", false, "message", optProp(response, "detail"), "queue", true, "endpoint", endpoint, "fn_index", fnIndex, "time", jsNow()))
		return false, nil
	}
	eventID := optProp(response, "event_id")

	var complete *gradioObject
	var pendingDiff []any
	connection, _ := dependency.get("connection").(string)

	// handle is the per-event callback. It returns true when close() runs.
	handle := func(raw *gradioObject) (closedNow bool) {
		defer func() {
			if r := recover(); r != nil {
				fire(statusEvent("type", "status", "stage", "error", "message", "An Unexpected Error Occurred!", "queue", true, "endpoint", endpoint, "fn_index", fnIndex, "time", jsNow()))
				closedNow = true
			}
		}()
		kind, status2, data2, originalMsg := handleMessage(raw)
		if kind == "heartbeat" {
			return false
		}
		switch {
		case kind == "update" && status2 != nil && complete == nil:
			m := obj("type", "status", "endpoint", endpoint, "fn_index", fnIndex, "time", jsNow())
			if originalMsg != undefined {
				m.set("original_msg", originalMsg)
			}
			spread(m, status2)
			fire(statusEventFrom(m))
		case kind == "complete":
			complete = status2
		case kind == "unexpected_error" || kind == "broken_connection":
			message := status2.get("message")
			if !isTruthy(message) {
				message = "An Unexpected Error Occurred!"
			}
			fire(statusEvent("type", "status", "stage", "error", "message", message, "queue", true, "endpoint", endpoint,
				"broken", kind == "broken_connection", "session_not_found", status2.get("session_not_found"), "fn_index", fnIndex, "time", jsNow()))
		case kind == "log":
			return false
		case kind == "generating" || kind == "streaming":
			m := obj("type", "status", "time", jsNow())
			spread(m, status2)
			m.set("stage", status2.get("stage"))
			m.set("queue", true)
			m.set("endpoint", endpoint)
			m.set("fn_index", fnIndex)
			fire(statusEventFrom(m))
			if data2 != nil && connection != "stream" && protocol != "sse_v1" {
				pendingDiff = applyDiffStream(pendingDiff, data2)
			}
		}
		if data2 != nil {
			outData, err := c.handlePayloadChecked(prop(data2, "data"))(dependency)
			if err != nil {
				panic(err)
			}
			fire(gradioEvent{kind: "data", msg: obj("type", "data", "time", jsNow(), "data", outData, "endpoint", endpoint, "fn_index", fnIndex)})
			if complete != nil {
				m := obj("type", "status", "time", jsNow())
				spread(m, complete)
				m.set("stage", status2.get("stage"))
				m.set("queue", true)
				m.set("endpoint", endpoint)
				m.set("fn_index", fnIndex)
				fire(statusEventFrom(m))
				return true
			}
		}
		return false
	}

	streamErr := c.openStream(ctx, func(raw *gradioObject) bool {
		if raw.get("msg") == "close_stream" {
			return true
		}
		id := raw.get("event_id")
		if !isTruthy(id) || id == eventID {
			if handle(raw) {
				closed = true
			}
		}
		return closed || stop
	})
	if streamErr != nil && !closed && !stop {
		// stream2.onerror reports a broken connection to every callback.
		if handle(obj("msg", "broken_connection", "message", brokenConnectionMsg)) {
			closed = true
		}
	}
	return closed, nil
}

func statusEventFrom(m *gradioObject) gradioEvent {
	stage, _ := m.get("stage").(string)
	return gradioEvent{kind: "status", stage: stage, msg: m}
}

// spread copies src's keys into dst, like {...dst, ...src}.
func spread(dst, src *gradioObject) {
	if src == nil {
		return
	}
	for _, k := range src.ownKeys() {
		dst.set(k, src.vals[k])
	}
}

func ensureSlash(endpoint string) string {
	if strings.HasPrefix(endpoint, "/") {
		return endpoint
	}
	return "/" + endpoint
}

// jsNow is new Date() as JSON.stringify prints it.
func jsNow() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }

func (c *gradioClient) skipQueue(fnIndex float64, dependency *gradioObject) bool {
	if q := dependency.get("queue"); q != nil && q != undefined {
		return !isTruthy(q)
	}
	return !isTruthy(c.config.get("enable_queue"))
}

// postData is post_data: it returns the parsed body and the status, or a
// broken-connection error with status 500.
func (c *gradioClient) postData(ctx context.Context, target string, body *gradioObject) (any, int) {
	text, _ := stringifyJSON(body)
	req, err := c.newRequest(ctx, http.MethodPost, target, strings.NewReader(text))
	if err != nil {
		return obj("error", brokenConnectionMsg), 500
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := gradioHTTP.Do(req)
	if err != nil {
		return obj("error", brokenConnectionMsg), 500
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return obj("error", "Could not parse server response: "+err.Error()), 500
	}
	v, err := parseJSON(raw)
	if err != nil {
		return obj("error", "Could not parse server response: SyntaxError"), 500
	}
	return v, resp.StatusCode
}

// openStream reads the queue's server-sent events until onMessage returns true,
// the server ends the stream, or ctx ends.
func (c *gradioClient) openStream(ctx context.Context, onMessage func(*gradioObject) bool) error {
	target := c.root + c.apiPrefix + "/queue/data?" + url.Values{"session_hash": {c.sessionHash}}.Encode()
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := c.newRequest(streamCtx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	resp, err := gradioHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("queue stream answered %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	var data *string
	for {
		line, err := readSSELine(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if line == "" {
			if data != nil {
				v, perr := parseJSON([]byte(*data))
				data = nil
				if m, ok := v.(*gradioObject); perr == nil && ok && onMessage(m) {
					return nil
				}
			}
			continue
		}
		field, value, ok := splitSSEField(line)
		if !ok || field != "data" {
			continue
		}
		if data == nil {
			data = &value
		} else {
			joined := *data + "\n" + value
			data = &joined
		}
	}
}

// readSSELine reads one line ending in \n, \r\n or a lone \r.
func readSSELine(r *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		c, err := r.ReadByte()
		if err != nil {
			if b.Len() > 0 && errors.Is(err, io.EOF) {
				return b.String(), nil
			}
			return "", err
		}
		switch c {
		case '\n':
			return b.String(), nil
		case '\r':
			if next, err := r.Peek(1); err == nil && next[0] == '\n' {
				_, _ = r.ReadByte()
			}
			return b.String(), nil
		}
		b.WriteByte(c)
	}
}

// splitSSEField is split(): the field must not be empty, and spaces after the
// colon are dropped.
func splitSSEField(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return "", "", false
	}
	return line[:i], strings.TrimLeft(line[i+1:], " \t\n\r\f\v"), true
}

// handleMessage is handle_message. It returns the type, status, data and
// original_msg.
func handleMessage(d *gradioObject) (string, *gradioObject, *gradioObject, any) {
	output, _ := d.get("output").(*gradioObject)
	success := isTruthy(d.get("success"))
	switch d.get("msg") {
	case "send_data":
		return "data", nil, nil, undefined
	case "send_hash":
		return "hash", nil, nil, undefined
	case "queue_full":
		return "update", obj("queue", true, "message", queueFullMsg, "stage", "error", "code", d.get("code"), "success", d.get("success")), nil, undefined
	case "heartbeat":
		return "heartbeat", nil, nil, undefined
	case "unexpected_error":
		return "unexpected_error", obj("queue", true, "message", d.get("message"), "session_not_found", d.get("session_not_found"), "stage", "error", "success", false), nil, undefined
	case "broken_connection":
		return "broken_connection", obj("queue", true, "message", d.get("message"), "stage", "error", "success", false), nil, undefined
	case "estimation":
		return "update", obj("queue", true, "stage", "pending", "code", d.get("code"), "size", d.get("queue_size"),
			"position", d.get("rank"), "eta", d.get("rank_eta"), "success", d.get("success")), nil, undefined
	case "progress":
		return "update", obj("queue", true, "stage", "pending", "code", d.get("code"), "progress_data", d.get("progress_data"), "success", d.get("success")), nil, undefined
	case "log":
		return "log", nil, d, undefined
	case "process_generating":
		var message, changed any = nil, undefined
		stage := "error"
		var data *gradioObject
		if success {
			stage = "generating"
			changed = output.get("changed_state_ids")
			data = output
		} else {
			message = mustProp(output, "error")
		}
		return "generating", obj("queue", true, "message", message, "stage", stage, "code", d.get("code"),
			"progress_data", d.get("progress_data"), "eta", d.get("average_duration"), "changed_state_ids", changed), data, undefined
	case "process_streaming":
		return "streaming", obj("queue", true, "message", mustProp(output, "error"), "stage", "streaming", "time_limit", d.get("time_limit"),
			"code", d.get("code"), "progress_data", d.get("progress_data"), "eta", d.get("eta")), output, undefined
	case "process_completed":
		if output == nil {
			panic(typeError("Cannot use 'in' operator to search for 'error'"))
		}
		if output.has("error") {
			return "update", obj("queue", true, "title", output.get("title"), "message", output.get("error"), "visible", output.get("visible"),
				"duration", output.get("duration"), "stage", "error", "code", d.get("code"), "success", d.get("success")), nil, undefined
		}
		var message, changed any = undefined, undefined
		stage := "error"
		var data *gradioObject
		if success {
			stage = "complete"
			changed = output.get("changed_state_ids")
			data = output
		} else {
			message = output.get("error")
		}
		return "complete", obj("queue", true, "message", message, "stage", stage, "code", d.get("code"),
			"progress_data", d.get("progress_data"), "changed_state_ids", changed), data, undefined
	case "process_starts":
		return "update", obj("queue", true, "stage", "pending", "code", d.get("code"), "size", d.get("rank"),
			"position", 0.0, "success", d.get("success"), "eta", d.get("eta")), nil, "process_starts"
	}
	return "none", obj("stage", "error", "queue", true), nil, undefined
}

func mustProp(o *gradioObject, key string) any {
	if o == nil {
		panic(typeError("Cannot read properties of undefined (reading '%s')", key))
	}
	return o.get(key)
}

// mapDataToParams is map_data_to_params.
func mapDataToParams(data any, endpointInfo *gradioObject) ([]any, error) {
	var params []any
	if endpointInfo != nil {
		params, _ = endpointInfo.get("parameters").([]any)
	}
	if arr, ok := data.([]any); ok {
		return arr, nil
	}
	if data == undefined {
		return []any{}, nil
	}
	view, err := objectView(data)
	if err != nil {
		return nil, err
	}
	resolved := make([]any, len(params))
	for i, p := range params {
		param, _ := p.(*gradioObject)
		name, _ := param.get("parameter_name").(string)
		switch {
		case param.get("parameter_name") != nil && view.has(name):
			resolved[i] = view.get(name)
		case isTruthy(param.get("parameter_has_default")):
			resolved[i] = param.get("parameter_default")
		default:
			return nil, fmt.Errorf("No value provided for required parameter: %v", param.get("parameter_name"))
		}
	}
	for _, key := range view.ownKeys() {
		found := false
		for _, p := range params {
			if param, _ := p.(*gradioObject); param.get("parameter_name") == key {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Parameter `%s` is not a valid keyword argument. Please refer to the API for usage.", key)
		}
	}
	for i, v := range resolved {
		param, _ := params[i].(*gradioObject)
		if v == undefined && !isTruthy(param.get("parameter_has_default")) {
			return nil, fmt.Errorf("No value provided for required parameter: %v", param.get("parameter_name"))
		}
	}
	return resolved, nil
}

// blobRef is one Blob or Buffer found by walk_and_store_blobs.
type blobRef struct {
	path []any // keys and indexes from the root
	data []byte
	kind string // component type, or ""
}

// handleBlob uploads every Blob and Buffer in data and replaces it, like
// handle_blob. data is changed in place and returned.
func (c *gradioClient) handleBlob(ctx context.Context, data []any, endpointInfo *gradioObject) ([]any, error) {
	var refs []blobRef
	var params []any
	if endpointInfo != nil {
		params, _ = endpointInfo.get("parameters").([]any)
	}
	for i, v := range data {
		kind := ""
		if i < len(params) {
			if p, ok := params[i].(*gradioObject); ok {
				kind, _ = p.get("component").(string)
			}
		}
		refs = append(refs, walkBlobs(v, kind, []any{i})...)
	}
	type uploaded struct {
		ref blobRef
		url any
	}
	var results []uploaded
	for _, ref := range refs {
		fileURL, err := c.uploadFiles(ctx, ref.data)
		if err != nil {
			return nil, err
		}
		results = append(results, uploaded{ref, fileURL})
	}
	root := any(data)
	for _, r := range results {
		switch {
		case r.ref.kind == "Gallery":
			root = updateObject(root, r.ref.path, r.url)
		case r.url != undefined:
			root = updateObject(root, r.ref.path, obj("path", r.url, "meta", obj("_type", "gradio.FileData")))
		}
	}
	return root.([]any), nil
}

func walkBlobs(v any, kind string, path []any) []blobRef {
	switch t := v.(type) {
	case []any:
		var refs []blobRef
		for i, e := range t {
			refs = append(refs, walkBlobs(e, kind, appendPath(path, i))...)
		}
		return refs
	case jsBlob:
		return []blobRef{{path: path, data: t.data, kind: kind}}
	case jsBuffer:
		return []blobRef{{path: path, data: t.data, kind: kind}}
	case *gradioObject:
		var refs []blobRef
		for _, k := range t.ownKeys() {
			refs = append(refs, walkBlobs(t.vals[k], "", appendPath(path, k))...)
		}
		return refs
	}
	return nil
}

func appendPath(path []any, key any) []any {
	return append(append([]any(nil), path...), key)
}

// updateObject sets the value at path, like update_object.
func updateObject(root any, path []any, value any) any {
	if len(path) == 0 {
		return value
	}
	switch t := root.(type) {
	case []any:
		i := path[0].(int)
		t[i] = updateObject(t[i], path[1:], value)
		return t
	case *gradioObject:
		k := path[0].(string)
		t.set(k, updateObject(t.get(k), path[1:], value))
		return t
	}
	return root
}

// uploadFiles posts one file to /upload and returns its server path, or
// undefined when the server refuses it. A network failure is an error.
func (c *gradioClient) uploadFiles(ctx context.Context, data []byte) (any, error) {
	var form bytes.Buffer
	w := multipart.NewWriter(&form)
	part, err := w.CreateFormFile("files", "blob")
	if err != nil {
		return nil, err
	}
	_, _ = part.Write(data)
	_ = w.Close()

	req, err := c.newRequest(ctx, http.MethodPost, c.root+c.apiPrefix+"/upload", &form)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := gradioHTTP.Do(req)
	if err != nil {
		return nil, errors.New(brokenConnectionMsg + err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return undefined, nil
	}
	v, err := parseJSON(body)
	if err != nil {
		return nil, err
	}
	if files, ok := v.([]any); ok && len(files) > 0 {
		return files[0], nil
	}
	return undefined, nil
}

// handlePayload is handle_payload. For outputs without null state, state
// components are dropped.
func (c *gradioClient) handlePayload(payload []any, dependency *gradioObject, kind string, withNullState bool) []any {
	key := "outputs"
	if kind == "input" {
		key = "inputs"
	}
	deps, _ := dependency.get(key).([]any)
	out := []any{}
	at := func(i int) any {
		if i < len(payload) {
			return payload[i]
		}
		return undefined
	}
	next := 0
	for _, id := range deps {
		if c.componentType(id) == "state" {
			if withNullState {
				if len(payload) == len(deps) {
					out = append(out, at(next))
					next++
				} else {
					out = append(out, nil)
				}
			} else {
				next++
			}
			continue
		}
		out = append(out, at(next))
		next++
	}
	return out
}

// handlePayloadChecked converts output data for an event. Data that is not an
// array fails like resolved_payload[i] would.
func (c *gradioClient) handlePayloadChecked(v any, err error) func(*gradioObject) ([]any, error) {
	return func(dependency *gradioObject) ([]any, error) {
		if err != nil {
			return nil, err
		}
		switch t := v.(type) {
		case []any:
			return c.handlePayload(t, dependency, "output", false), nil
		case nil, undefinedType:
			outputs, _ := dependency.get("outputs").([]any)
			if len(outputs) == 0 {
				return []any{}, nil
			}
			return nil, typeError("Cannot read properties of %s (reading '0')", jsTypeName(v))
		case *gradioObject:
			var arr []any
			outputs, _ := dependency.get("outputs").([]any)
			for i := range outputs {
				arr = append(arr, t.get(strconv.Itoa(i)))
			}
			return c.handlePayload(arr, dependency, "output", false), nil
		}
		return c.handlePayload(nil, dependency, "output", false), nil
	}
}

// applyDiffStream is apply_diff_stream for one event.
func applyDiffStream(pending []any, data *gradioObject) []any {
	values, _ := data.get("data").([]any)
	if pending == nil {
		return append([]any{}, values...)
	}
	for i, diff := range values {
		var current any = undefined
		if i < len(pending) {
			current = pending[i]
		}
		updated := applyDiff(current, diff)
		for len(pending) <= i {
			pending = append(pending, undefined)
		}
		pending[i] = updated
		values[i] = updated
	}
	return pending
}

func applyDiff(target, diff any) any {
	edits, _ := diff.([]any)
	for _, e := range edits {
		edit, _ := e.([]any)
		if len(edit) < 3 {
			panic(typeError("invalid diff"))
		}
		action, _ := edit[0].(string)
		path, _ := edit[1].([]any)
		target = applyEdit(target, path, action, edit[2])
	}
	return target
}

func applyEdit(target any, path []any, action string, value any) any {
	if len(path) == 0 {
		switch action {
		case "replace":
			return value
		case "append":
			return jsAdd(target, value)
		}
		panic(fmt.Errorf("Unsupported action: %s", action))
	}
	current := target
	for _, key := range path[:len(path)-1] {
		next, err := prop(current, jsKey(key))
		if err != nil {
			panic(err)
		}
		current = next
	}
	last := jsKey(path[len(path)-1])
	switch t := current.(type) {
	case *gradioObject:
		switch action {
		case "replace", "add":
			t.set(last, value)
		case "append":
			t.set(last, jsAdd(t.get(last), value))
		case "delete":
			t.delete(last)
		default:
			panic(fmt.Errorf("Unknown action: %s", action))
		}
	case []any:
		i, _ := strconv.Atoi(last)
		switch action {
		case "replace":
			if i < len(t) {
				t[i] = value
			}
		case "append":
			if i < len(t) {
				t[i] = jsAdd(t[i], value)
			}
		case "add":
			// Arrays grow through their parent, so rebuild it there.
			panic(typeError("adding to an array element is not supported by the Go port"))
		case "delete":
			panic(typeError("deleting from an array element is not supported by the Go port"))
		default:
			panic(fmt.Errorf("Unknown action: %s", action))
		}
	default:
		panic(typeError("Cannot set properties of %s", jsTypeName(current)))
	}
	return target
}

func jsKey(k any) string {
	switch t := k.(type) {
	case string:
		return t
	case float64:
		return jsNumberString(t)
	}
	s, _ := stringifyJSON(k)
	return s
}

// jsAdd is a + b for the value types diffs carry.
func jsAdd(a, b any) any {
	if x, ok := a.(float64); ok {
		if y, ok := b.(float64); ok {
			return x + y
		}
	}
	return jsToString(a) + jsToString(b)
}

func jsToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return jsNumberString(t)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return "null"
	case undefinedType:
		return "undefined"
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			if e != nil && e != undefined {
				parts[i] = jsToString(e)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

// deepCopy snapshots an event's data. The original hands out shared objects
// that later diffs change, which [name].tsx works around with a JSON round trip.
func deepCopy(v any) []any {
	s, _ := stringifyJSON(v)
	parsed, err := parseJSON([]byte(s))
	if err != nil {
		return nil
	}
	arr, _ := parsed.([]any)
	return arr
}
