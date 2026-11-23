from . import transport
from .gradio_resolve import resolve

GET = "GET"
POST = "POST"
STREAM = "STREAM"


class SubBridge:
    """One area of the Go core (history, ...), reached as ``bridge.<name>``.

    A subclass lists its operations in ``ops``: name -> (verb, parameter names).
    ``bridge.history.copy(directory, to)`` then calls ``POST /api/history/copy``
    with ``{"directory": ..., "to": ...}`` (GET sends a query string). Arguments
    may be positional or keyword, so a bound operation can be a Gradio event
    handler directly. An operation name maps to its endpoint with ``_`` -> ``-``.

    STREAM is a POST whose answer is a stream of output lines (an install). The
    operation is a generator that yields the console HTML so far (lines joined
    with ``<br />``) after each line, so Gradio treats it as a streaming handler.

    Go answers with exactly the value the caller needs, and Gradio markers in
    it come back as components. There is no Python fallback: without the core
    a call raises CoreBridgeError.
    """

    name = ""
    ops = {}

    def __getattr__(self, op):
        ops = type(self).ops
        if op.startswith("_") or op not in ops:
            raise AttributeError(
                f"{type(self).__name__} has no operation {op!r}; known: {sorted(ops)}"
            )
        verb, params = ops[op]
        path = f"/api/{self.name}/{op.replace('_', '-')}"

        def fields_of(args, kwargs):
            if len(args) > len(params):
                raise TypeError(
                    f"{op}() takes {len(params)} arguments ({len(args)} given)"
                )
            fields = dict(zip(params, args))
            for k, v in kwargs.items():
                if k not in params or k in fields:
                    raise TypeError(
                        f"{op}() got an unexpected or repeated argument {k!r}"
                    )
                fields[k] = v
            missing = [p for p in params if p not in fields]
            if missing:
                raise TypeError(f"{op}() missing arguments: {missing}")
            return fields

        if verb == STREAM:

            def call(*args, **kwargs):
                lines = []
                for rec in transport.stream(path, body=fields_of(args, kwargs)):
                    if "line" in rec:
                        lines.append(rec["line"])
                        yield "<br />".join(lines)

        else:

            def call(*args, **kwargs):
                fields = fields_of(args, kwargs)
                if verb == GET:
                    return resolve(transport.call("GET", path, query=fields or None))
                return resolve(transport.call("POST", path, body=fields))

        call.__name__ = op
        return call
