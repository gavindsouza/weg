package output

import "strings"

// Diff renders a simple line-based diff between old and new content.
// Lines only in old are prefixed with "-", lines only in new with "+",
// and unchanged lines are shown with a leading space.
func Diff(oldContent, newContent string) string {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	lcs := longestCommonSubsequence(oldLines, newLines)
	oi, ni := 0, 0

	var out strings.Builder
	for _, common := range lcs {
		for oi < len(oldLines) && oldLines[oi] != common {
			out.WriteString("-" + oldLines[oi] + "\n")
			oi++
		}
		for ni < len(newLines) && newLines[ni] != common {
			out.WriteString("+" + newLines[ni] + "\n")
			ni++
		}
		if oi < len(oldLines) {
			out.WriteString(" " + oldLines[oi] + "\n")
			oi++
			ni++
		}
	}
	for oi < len(oldLines) {
		out.WriteString("-" + oldLines[oi] + "\n")
		oi++
	}
	for ni < len(newLines) {
		out.WriteString("+" + newLines[ni] + "\n")
		ni++
	}
	return out.String()
}

// DiffEmpty reports whether two contents are identical.
func DiffEmpty(oldContent, newContent string) bool {
	return oldContent == newContent
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// longestCommonSubsequence returns the LCS of two line slices.
func longestCommonSubsequence(a, b []string) []string {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] > dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var seq []string
	for i, j := 0, 0; i < n && j < m; {
		if a[i] == b[j] {
			seq = append(seq, a[i])
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return seq
}