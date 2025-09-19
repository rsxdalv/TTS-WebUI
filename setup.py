import setuptools
from pathlib import Path

# Define versions
TORCH_VERSION = "2.7.0"
CUDA_VERSION = "cu128"
torch = f"torch=={TORCH_VERSION}"

HERE = Path(__file__).parent
with open(HERE / "requirements.txt") as f:
    requirements = f.read().splitlines()

# Define optional dependencies
extras_require = {
    "cpu": [    
        torch,
        "torchvision",
        "torchaudio",
    ],
    "cuda": [
        f"{torch}+{CUDA_VERSION}",
        # f"torch=={TORCH_VERSION} --index-url https://download.pytorch.org/whl/{CUDA_VERSION}",
        "torchvision",
        "torchaudio",
        "xformers",
        # "xformers>=0.0.20",
        # "triton>=2.0.0",
        # "flash-attn>=2.0.0",
    ],
    "mac": [
        torch,
        "torchvision",
        "torchaudio",
    ],
    "rocm": [
        torch,
        "torchvision",
        "torchaudio",
    ],
    "intel": [
        torch,
        "torchvision",
        "torchaudio",
    ],
}

setuptools.setup(
    name="TTS-WebUI",
    version="0.0.1",
    # packages=[],
    packages=["tts_webui"],
    # packages=setuptools.find_namespace_packages(),
    # packages=setuptools.find_namespace_packages(
    #     include=[
    #         "tts_webui.*",
    #     ]
    # ),
    # include_package_data=True,
    package_data={"": ["*.json"]},
    author="rsxdalv",
    description="TTS WebUI / Harmonica",
    long_description=open(HERE / "README.md", "r", encoding="utf-8").read(),
    long_description_content_type="text/markdown",
    url="https://github.com/rsxdalv/tts-webui",
    project_urls={},
    entry_points={
        "console_scripts": [
            "tts-webui=tts_webui.cli:main",
        ],
    },
    install_requires=requirements,
    extras_require=extras_require,
    dependency_links=[
        # "https://download.pytorch.org/whl/cu128",
    ],
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
)
