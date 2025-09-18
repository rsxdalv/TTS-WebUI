import setuptools

# Define versions
TORCH_VERSION = "2.7.0"
CUDA_VERSION = "cu128"

with open("requirements.txt") as f:
    requirements = f.read().splitlines()

# Define optional dependencies
extras_require = {
    "cpu": [
        f"torch=={TORCH_VERSION}",
        "torchvision",
        "torchaudio",
    ],
    "cuda": [
        # "xformers>=0.0.20",
        # "triton>=2.0.0",
        # "flash-attn>=2.0.0",
        # f"torch=={TORCH_VERSION} --index-url https://download.pytorch.org/whl/{CUDA_VERSION}",
        f"torch=={TORCH_VERSION}+{CUDA_VERSION}",
        "torchvision",
        "torchaudio",
        "xformers",
    ],
    "mac": [
        f"torch=={TORCH_VERSION}",
        "torchvision",
        "torchaudio",
    ],
    "rocm": [
        f"torch=={TORCH_VERSION}",
        "torchvision",
        "torchaudio",
        # "torch>=2.0.0",
        # Add any ROCM specific packages here
    ],
    "intel": [
        f"torch=={TORCH_VERSION}",
        "torchvision",
        "torchaudio",
        # --index-url https://download.pytorch.org/whl/test/xpu
    ],
}

setuptools.setup(
    # name="tts_webui",
    name="tts_webui_deps",
    # packages=setuptools.find_namespace_packages(),
    # packages=["tts_webui"],
    # packages=setuptools.find_namespace_packages(
    #     include=[
    #         "tts_webui.*",
    #     ]
    # ),
    # include_package_data=True,
    packages=[],
    version="0.5.0",
    author="rsxdalv",
    description="TTS WebUI / Harmonica",
    url="https://github.com/rsxdalv/tts-webui",
    project_urls={},
    scripts=[],
    # install_requires=[
    #     "extension_kokoro @ git+https://github.com/rsxdalv/extension_kokoro@main",
    #     "extension_rvc @ git+https://github.com/rsxdalv/extension_rvc@main",
    #     "openai",
    # ],
    # install_requires=requirements,
    # install_requires=[],
    install_requires=requirements,
    extras_require=extras_require,
    dependency_links=[
        # "https://download.pytorch.org/whl/cu128",
    ],
    package_data={"": ["*.json"]},
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
)
