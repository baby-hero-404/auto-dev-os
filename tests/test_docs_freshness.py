import os
import sys
import unittest
from unittest.mock import patch

# Add scripts directory to path to import docs_freshness
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../scripts')))
import docs_freshness

class TestDocsFreshness(unittest.TestCase):
    def test_parse_frontmatter_valid(self):
        content = "---\nsources:\n  - server/**\nverified: 2026-07-21\n---\n# Title"
        fm, rest = docs_freshness.parse_frontmatter(content)
        self.assertIsNotNone(fm)
        self.assertIn('sources', fm)
        self.assertEqual(fm['sources'], ['server/**'])
        self.assertEqual(fm['verified'], '2026-07-21')
        self.assertEqual(rest, '# Title')

    def test_parse_frontmatter_invalid(self):
        content = "# No frontmatter here\nsources: server/**"
        fm, rest = docs_freshness.parse_frontmatter(content)
        self.assertIsNone(fm)
        self.assertEqual(rest, content)

    @patch('docs_freshness.subprocess.run')
    def test_run_git_log_success(self, mock_run):
        mock_run.return_value.stdout = "1672531200\n" # Some timestamp
        ts = docs_freshness.run_git_log("some/path")
        self.assertEqual(ts, 1672531200)
        
    @patch('docs_freshness.subprocess.run')
    def test_run_git_log_fail(self, mock_run):
        mock_run.side_effect = docs_freshness.subprocess.CalledProcessError(1, 'cmd')
        ts = docs_freshness.run_git_log("some/path")
        self.assertEqual(ts, 0)

if __name__ == '__main__':
    unittest.main()
