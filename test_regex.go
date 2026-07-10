package main
import (
	"fmt"
	"regexp"
	"strings"
)
var uuidRegex = regexp.MustCompile(`(?i)([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
func main() {
	line := "I0710 10:40:45.596093 1809104 server.go:861] Created conversation 23821d73-709c-4501-af10-cc5aeece2e14"
	if strings.Contains(line, "Created conversation") {
		matches := uuidRegex.FindStringSubmatch(line)
		fmt.Printf("Matches: %q\n", matches)
	}
}
