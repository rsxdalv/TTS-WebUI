from . import transport


class SubBridge:
    """One area of the Go core (history, ...), reached as ``bridge.<name>``.

    Subclasses are written by hand: each public method calls a Go endpoint
    (``_remote``) and returns what the handler needs, resolved Gradio objects
    included. ``/api/<name>/<op>`` is the endpoint convention. There is no
    Python fallback: without the core the call raises CoreBridgeError.
    """

    name = ""

    def _remote(self, op, body=None, query=None):
        method = "GET" if body is None else "POST"
        return transport.call(method, f"/api/{self.name}/{op}", query=query, body=body)
