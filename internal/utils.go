package internal

import "strings"

func unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func extractSecondLevelDomain(domain string) string {
	segments := strings.Split(domain, ".")
	if len(segments) < 2 {
		return ""
	}
	return strings.Join(segments[len(segments)-2:], ".")
}
