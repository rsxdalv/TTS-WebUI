from . import transport


class SubBridge:
    """One area of the Go core (history, ...), reached as ``bridge.<name>``.

    Subclasses are written by hand: each public method says which Go endpoint
    it uses (``_remote``) and what plain Python does when the core is off.
    ``/api/<name>/<op>`` is the endpoint convention.
    """

    name = ""

    @property
    def enabled(self):
        return transport.enabled()

    def _remote(self, op, body=None, query=None):
        method = "GET" if body is None else "POST"
        return transport.call(method, f"/api/{self.name}/{op}", query=query, body=body)
