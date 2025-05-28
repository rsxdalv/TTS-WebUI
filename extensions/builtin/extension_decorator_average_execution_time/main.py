import time


def extension__tts_generation_webui():
    return {
        "package_name": "extension_decorator_average_execution_time",
        "name": "Decorator Average Execution Time",
        "requirements": "git+https://github.com/rsxdalv/extension_decorator_average_execution_time@main",
        "description": "Decorator Average Execution Time",
        "extension_type": "decorator",
        "extension_class": "outer",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_decorator_average_execution_time",
        "extension_website": "https://github.com/rsxdalv/extension_decorator_average_execution_time",
        "extension_platform_version": "0.0.1",
    }


def decorator_average_execution_time(fn):
    times = []

    def wrapper(*args, **kwargs):
        start = time.time()
        result = fn(*args, **kwargs)
        end = time.time()
        times.append(end - start)
        print(f"Average execution time: {sum(times) / len(times):.3f}")
        return result

    return wrapper
