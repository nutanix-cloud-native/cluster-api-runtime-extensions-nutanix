// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentialprovider

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
)

// ParseSchemelessURL parses a schemeless url and returns a url.URL
// url.Parse require a scheme, but ours don't have schemes.  Adding a
// scheme to make url.Parse happy, then clear out the resulting scheme.
func ParseSchemelessURL(schemelessURL string) (*url.URL, error) {
	parsed, err := url.Parse("https://" + schemelessURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	// clear out the resulting scheme
	parsed.Scheme = ""
	return parsed, nil
}

// SplitURL splits the host name into parts, as well as the port.
func SplitURL(u *url.URL) (parts []string, port string) {
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		// could not parse port
		host, port = u.Host, ""
	}
	return strings.Split(host, "."), port
}

// URLsMatchStr is wrapper for URLsMatch, operating on strings instead of URLs.
func URLsMatchStr(glob, target string) (bool, error) {
	globURL, err := ParseSchemelessURL(glob)
	if err != nil {
		return false, err
	}
	targetURL, err := ParseSchemelessURL(target)
	if err != nil {
		return false, err
	}
	return URLsMatch(globURL, targetURL)
}

// URLsMatch checks whether the given target url matches the glob url, which may have
// glob wild cards in the host name.
//
// Examples:
//
//	globURL=*.docker.io, targetURL=blah.docker.io => match
//	globURL=*.docker.io, targetURL=not.right.io   => no match
//
// Note that we don't support wildcards in ports and paths yet.
func URLsMatch(globURL, targetURL *url.URL) (bool, error) {
	globURLParts, globPort := SplitURL(globURL)
	targetURLParts, targetPort := SplitURL(targetURL)
	if globPort != targetPort {
		// port doesn't match
		return false, nil
	}
	if len(globURLParts) != len(targetURLParts) {
		// host name does not have the same number of parts
		return false, nil
	}
	if !strings.HasPrefix(targetURL.Path, globURL.Path) {
		// the path of the credential must be a prefix
		return false, nil
	}
	for k, globURLPart := range globURLParts {
		targetURLPart := targetURLParts[k]
		matched, err := filepath.Match(globURLPart, targetURLPart)
		if err != nil {
			return false, fmt.Errorf("bad glob pattern: %w", err)
		}
		if !matched {
			// glob mismatch for some part
			return false, nil
		}
	}
	// everything matches
	return true, nil
}
