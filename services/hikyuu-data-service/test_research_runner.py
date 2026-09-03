import unittest
from unittest.mock import patch

import research_runner


def sample_rows(count=40):
    return [
        {
            "time": f"2024-01-{index + 1:02d}T00:00:00+08:00",
            "open": 10 + index * 0.1,
            "high": 10.2 + index * 0.1,
            "low": 9.8 + index * 0.1,
            "close": 10 + index * 0.1,
            "volume": 1000 + index,
            "amount": 10000 + index,
        }
        for index in range(count)
    ]


class ResearchRunnerTest(unittest.TestCase):
    @patch.object(research_runner, "_indicator_from_hikyuu", return_value=None)
    @patch.object(research_runner, "load_records", return_value={"list": sample_rows()})
    def test_indicator_response_contains_revision_metadata(self, _load, _native):
        result = research_runner.calculate_indicator({"code": "000001", "indicator": "macd", "limit": 40})
        self.assertEqual(result["indicator"], "macd")
        self.assertEqual(result["count"], 40)
        self.assertEqual(result["meta"]["calculation_engine"], "native-fallback")
        self.assertEqual(len(result["list"]), 40)

    @patch.object(research_runner, "load_records", return_value={"list": sample_rows()})
    def test_reference_backtest_is_reproducible(self, _load):
        result = research_runner.run_reference_backtest({"symbols": ["000001"], "history_count": 40})
        self.assertEqual(result["engine"], "hikyuu")
        self.assertEqual(result["meta"]["strategy"], "ma_cross_reference")
        self.assertIn("total_return", result["metrics"])


if __name__ == "__main__":
    unittest.main()
