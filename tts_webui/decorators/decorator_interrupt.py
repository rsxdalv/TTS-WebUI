import asyncio
import functools

import gradio as gr


class InterruptionFlag:
    def __init__(self):
        self._interrupted = False
        self._ack_event = asyncio.Event()

    def interrupt(self):
        self._interrupted = True
        # Do NOT set ack_event here — only acknowledge() should

    def reset(self):
        self._interrupted = False
        self._ack_event.clear()

    def is_interrupted(self):
        return self._interrupted

    def acknowledge(self):
        self._ack_event.set()

    async def join(self, timeout=None):
        """Wait until acknowledge() is called after interrupt()."""
        try:
            await asyncio.wait_for(self._ack_event.wait(), timeout)
        except asyncio.TimeoutError:
            raise RuntimeError("Timeout waiting for interruption to be acknowledged.")


_interruption_flags = {}


def get_interruption_flag(namespace) -> InterruptionFlag:
    return _interruption_flags.setdefault(namespace, InterruptionFlag())


async def interrupt(namespace):
    get_interruption_flag(namespace).interrupt()
    await get_interruption_flag(namespace).join()


def interrupt_callback_factory(namespace):
    async def callback():
        await interrupt(namespace)
        return "Interrupt next chunk"

    return callback


def InterruptButton(namespace, **kwargs):
    button = gr.Button("Interrupt next chunk", **kwargs)
    button.click(fn=lambda: gr.Button("Interrupting..."), outputs=[button]).then(
        fn=interrupt_callback_factory(namespace), outputs=button
    )
    return button


def interruptible(namespace):
    """
    Decorator to make a generator function interruptible via a namespace.
    """

    def decorator(gen_func):
        @functools.wraps(gen_func)
        def wrapper(*args, **kwargs):
            flag: InterruptionFlag = get_interruption_flag(namespace)
            flag.reset()

            gen = gen_func(*args, **kwargs)

            # If the result is not an iterator, return it as is
            # (This handles cases where the decorated function is not a generator)
            if not hasattr(gen, "__iter__") and not hasattr(gen, "__next__"):
                return gen

            try:
                for item in gen:
                    yield item
                    if flag.is_interrupted():
                        print(f"Interrupted: {namespace}")
                        break
            finally:
                flag.acknowledge()
                if hasattr(gen, "close"):
                    gen.close()

        return wrapper

    return decorator
