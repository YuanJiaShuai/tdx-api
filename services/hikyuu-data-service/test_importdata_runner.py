import os
import unittest
from unittest.mock import patch

import importdata_runner


class ImportDataRunnerTest(unittest.TestCase):
    def test_configured_hosts_uses_environment_override(self):
        with patch.dict(os.environ, {"HIKYUU_TDX_HOSTS": "127.0.0.1:7709,10.0.0.2:7710"}):
            self.assertEqual(
                importdata_runner.configured_hosts(),
                [("127.0.0.1", 7709), ("10.0.0.2", 7710)],
            )

    def test_configured_hosts_uses_defaults_for_empty_value(self):
        with patch.dict(os.environ, {"HIKYUU_TDX_HOSTS": ""}):
            self.assertEqual(
                importdata_runner.configured_hosts(),
                list(importdata_runner.DEFAULT_TDX_HOSTS),
            )


if __name__ == "__main__":
    unittest.main()
