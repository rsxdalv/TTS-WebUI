# TTS WebUI Testing Reference

## Overview

The TTS WebUI test suite ensures the reliability and stability of the application by focusing on critical functionality, core utilities, and integration points. The testing approach prioritizes quality over quantity, targeting essential paths and configurations. This document provides a detailed guide to the testing framework, tools, and methodologies used in the project.

---

## Quick Test Guide

### Running Tests
- **Run all tests**: Execute the entire test suite using `pytest`.
  ```bash
  pytest
  ```
- **Verbose output**: Use verbose mode for detailed logs.
  ```bash
  pytest -v
  ```
- **Coverage**: Measure code coverage during test execution.
  ```bash
  pytest --cov=tts_webui
  ```
- **Specific tests**: Run individual test files or cases.
  ```bash
  pytest tests/test_config.py
  ```
- **Filter by markers**: Use markers like `unit`, `integration`, or `slow` to filter tests.
  ```bash
  pytest -m "unit"
  ```

### Test Structure
- **Fixtures**: Shared fixtures are defined in `conftest.py` to handle common setup tasks.
- **Organization**: Test files are grouped by functionality:
  - `tests/test_config.py`: Configuration management tests.
  - `tests/test_utils.py`: Utility function tests.
  - `tests/test_decorators.py`: Decorator functionality tests.
  - `tests/integration/`: Integration tests for modules and CLI commands.

---

## Test Infrastructure

### Key Components
- **Configuration**: Core configuration files for test discovery and utilities are located in the `tests/` directory.
- **Markers**: Tests are categorized using markers such as `@pytest.mark.unit` and `@pytest.mark.integration`.
- **Output**: Test results are formatted for readability, and coverage reports are generated using `pytest-cov`.

### Tools Used
- **Pytest**: Primary testing framework.
- **Pytest-Cov**: For measuring code coverage.
- **Mock**: For mocking dependencies in unit tests.
- **Flake8**: For linting and ensuring code quality.

### Test Suites
- **Unit Tests**: Focus on individual functions and methods, ensuring they behave as expected in isolation.
- **Integration Tests**: Validate the interaction between different modules, including environment management and CLI commands.
- **End-to-End Tests**: Simulate real-world usage scenarios to ensure the application works as a whole.

---

## Getting Started

### Steps
1. **Install Dependencies**:
   Ensure all dependencies are installed by running:
   ```bash
   pip install -r requirements.txt
   ```
2. **Run Tests**:
   Execute the test suite using `pytest`.
3. **View Coverage**:
   Generate a coverage report using:
   ```bash
   pytest --cov=tts_webui --cov-report=html
   ```
   Open the `htmlcov/index.html` file in a browser to view detailed coverage information.

---

## Additional Notes

- For detailed logs, use the `-v` flag with `pytest`.
- To debug failing tests, use the `--pdb` flag to drop into the Python debugger on failure.
- Refer to the `tests/` directory for examples of well-structured test cases.

---
