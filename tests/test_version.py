from packaging.version import Version

from prefect_operator import __version__


def test_version_is_sensible():
    version = Version(__version__)
    assert version.major >= 0
    assert version.minor > 0
