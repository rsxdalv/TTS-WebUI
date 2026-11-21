from . import transport
from .gradio_resolve import resolve


class SubBridge:
    """One area of the Go core (history, ...), reached as ``bridge.<name>``.

    Subclasses are written by hand. A method forwards its arguments to the Go
    endpoint by name, so Python and Go use the same field names:

        def copy(self, directory, to):
            return self._remote("copy", directory=directory, to=to)["ui"]

    ``/api/<name>/<op>`` is the endpoint convention. Gradio markers in the
    answer come back as Gradio components. There is no Python fallback:
    without the core the call raises CoreBridgeError.
    """

    name = ""

    def _remote(self, op, **body):
        """POST the keyword arguments as JSON; return the resolved answer."""
        return resolve(transport.call("POST", f"/api/{self.name}/{op}", body=body))

    def _get(self, op, **query):
        """GET with the keyword arguments as the query string."""
        return resolve(transport.call("GET", f"/api/{self.name}/{op}", query=query))
