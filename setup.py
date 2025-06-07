import setuptools

# get requirements from requirements.txt
with open("requirements.txt") as f:
    requirements = f.read().splitlines()

# Define optional dependencies
extras_require = {
    "cuda": [
        # "xformers>=0.0.20",
        # "triton>=2.0.0",
        # "flash-attn>=2.0.0",
        "torch==2.6.0+cu126",
        # "torch==2.7.0+cu126",
    ],
    "rocm": [
        # "torch>=2.0.0",
        # Add any ROCM specific packages here
    ],
}

setuptools.setup(
    name="tts_webui",
    packages=setuptools.find_namespace_packages(),
    version="0.4.0",
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
    install_requires=requirements,
    # install_requires=[],
    extras_require=extras_require,
    include_package_data=True,
    # dependency_links=[
    #     "https://download.pytorch.org/whl/cu126",
    # ],
    package_data={"": ["*.json"]},
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
)
