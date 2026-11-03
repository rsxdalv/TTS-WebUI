import os
import random

import numpy as np


def set_seed(seed: int = 0):
    """Set the seed
    seed = 0         Generate a random seed
    seed = -1        Disable deterministic algorithms
    0 < seed < 2**32 Set the seed
    Args:
        seed: integer to use as seed
    Returns:
        integer used as seed
    """

    original_seed = seed

    try:
        import torch  # lazy — torch is optional (P5)
        _torch_available = True
    except ImportError:
        _torch_available = False

    # See for more informations: https://pytorch.org/docs/stable/notes/randomness.html
    if _torch_available:
        if seed == -1:
            # Disable deterministic
            torch.backends.cudnn.deterministic = False
            torch.backends.cudnn.benchmark = True
        else:
            # Enable deterministic
            torch.backends.cudnn.deterministic = True
            torch.backends.cudnn.benchmark = False

    if seed <= 0:
        # Generate random seed
        # Use default_rng() because it is independent of np.random.seed()
        seed = np.random.default_rng().integers(1, 2**32 - 1)

    seed = int(seed)

    assert 0 < seed < 2**32

    np.random.seed(seed)
    random.seed(seed)
    if _torch_available:
        torch.manual_seed(seed)
        torch.cuda.manual_seed_all(seed)
    os.environ["PYTHONHASHSEED"] = str(seed)

    return original_seed if original_seed != 0 else seed
