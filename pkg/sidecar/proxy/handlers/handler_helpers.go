package handlers

func GetPrefillHostPortFromHeader(headerValues []string) string {
	return headerValues[0]
}
