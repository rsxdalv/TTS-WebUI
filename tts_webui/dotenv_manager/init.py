import os

from .writer import generate_env, write_env


try:
    from dotenv import load_dotenv
except ImportError:
    print(
        "Missing core packages (dotenv), the app has not been installed successfully."
    )
    exit(1)


def init():
    if not os.path.exists(".env"):
        print("Env file not found. Creating default env.")
        write_env(generate_env())
    load_dotenv()
    load_dotenv(dotenv_path=".env.user", override=True)

    # print env
    for key, value in os.environ.items():
        # if key.startswith("HUGGINGFACE") or key.startswith("HF_") or key.startswith(
        #     "TORCH"
        # ) or key.startswith("XDG_") or key.startswith("USE_TF") or key.startswith(
        #     "weight_"
        # ):
        print(f"{key}={value}")
    print("")
