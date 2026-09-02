package utils

// ContainsFold reports whether substr is within s, using a case-insensitive
// match that does not allocate new strings.
func ContainsFold(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	first := substr[0]
	firstUpper := first
	if first >= 'a' && first <= 'z' {
		firstUpper = first - ('a' - 'A')
	} else if first >= 'A' && first <= 'Z' {
		firstUpper = first + ('a' - 'A')
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		c := s[i]
		if c != first && c != firstUpper {
			continue
		}
		match := true
		for j := 1; j < len(substr); j++ {
			c2 := s[i+j]
			c3 := substr[j]
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c3 >= 'A' && c3 <= 'Z' {
				c3 += 'a' - 'A'
			}

			if c2 != c3 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// HasPrefixFold reports whether the string s begins with prefix, using a case-insensitive
// match that does not allocate new strings.
func HasPrefixFold(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c2 := s[i]
		c3 := prefix[i]
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c3 >= 'A' && c3 <= 'Z' {
			c3 += 'a' - 'A'
		}
		if c2 != c3 {
			return false
		}
	}
	return true
}

// TruncateLines checks if the text has more than maxLines lines.
// If it does, it truncates the text to keep only the last maxLines lines
// and prepends "... [Logs Truncated] ...\n" to the result.
// It avoids allocating intermediate slices and arrays that strings.Split uses.
func TruncateLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return "... [Logs Truncated] ...\n"
	}
	count := 0
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '\n' {
			count++
			if count == maxLines {
				return "... [Logs Truncated] ...\n" + text[i+1:]
			}
		}
	}
	return text
}
