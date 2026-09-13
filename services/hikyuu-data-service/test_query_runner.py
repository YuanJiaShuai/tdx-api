import unittest
from unittest.mock import patch

import query_runner


class QueryRunnerBatchTest(unittest.TestCase):
    def test_batch_keeps_successful_symbols_and_reports_failures(self):
        def fake_load(symbol, period, start, end, limit, recover):
            if symbol == "000002":
                raise LookupError("no kline data")
            return {"symbol": symbol, "list": [{"close": 10.2}]}

        with patch.object(query_runner, "load_records", side_effect=fake_load):
            result = query_runner.load_records_batch(
                ["000001", "000002"], "day", "", "", 260, "qfq"
            )

        self.assertIn("000001", result["data"])
        self.assertEqual(result["errors"]["000002"], "no kline data")
        self.assertEqual(result["meta"]["source"], "hikyuu")


if __name__ == "__main__":
    unittest.main()
