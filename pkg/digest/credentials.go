package digest

import (
	"fmt"
	"strings"
)

// Credentials holds the per request response params
type Credentials struct {
	// our creds
	Username string
	Password string

	// from the challenge
	Realm      string
	Nonce      string
	NonceCount int // times we've responded to this nonce
	Opaque     string
	Qop        string // the chosen auth from the server list
	Algorithm  string // <alg>-sess implies session-keying ()

	// session-keying
	CnoncePrime string
	NoncePrime  string

	// per response
	Method string
	URI    string
	Body   string // used for auth-int
	Cnonce string
}

func (c *Credentials) Hasher() Hasher {
	alg := strings.ToUpper(strings.TrimSuffix(c.Algorithm, "-sess"))
	return Algs[alg]
}

func (c *Credentials) kd(secret, data string) string {
	return fmt.Sprintf("%s:%s", secret, data)
}

func (c *Credentials) a1() string {
	var a1 []string
	a1 = append(a1, c.Username)
	a1 = append(a1, c.Realm)
	a1 = append(a1, c.Password)
	if strings.HasSuffix(c.Algorithm, "-sess") {
		a1 = append(a1, c.NoncePrime)
		a1 = append(a1, c.CnoncePrime)
	}
	return strings.Join(a1, ":")
}

func (c *Credentials) a2(method, uri, body string) string {
	var a2 []string
	a2 = append(a2, method)
	a2 = append(a2, uri)
	if strings.HasSuffix(c.Qop, "-int") {
		h := c.Hasher()
		a2 = append(a2, h(body))
	}
	return strings.Join(a2, ":")
}

func (c *Credentials) resp() (string, error) {
	h := c.Hasher()
	ha1 := h(c.a1())
	ha2 := h(c.a2(c.Method, c.URI, c.Body))
	switch c.Qop {
	case "auth", "auth-int":
		var data []string
		data = append(data, c.Nonce)
		data = append(data, fmt.Sprintf("%08x", c.NonceCount))
		data = append(data, c.Cnonce)
		data = append(data, c.Qop)
		data = append(data, ha2)
		return h(c.kd(ha1, strings.Join(data, ":"))), nil
	case "": // compat with rfc2617
		var data []string
		data = append(data, c.Nonce)
		data = append(data, ha2)
		return h(c.kd(ha1, strings.Join(data, ":"))), nil
	default:
		return "", ErrQopNotSupported
	}
}

func (c *Credentials) Authorization() (string, error) {
	if _, ok := Algs[strings.ToUpper(c.Algorithm)]; !ok {
		return "", ErrAlgNotImplemented
	}
	resp, err := c.resp()
	if err != nil {
		return "", err
	}
	var auth []string
	auth = append(auth, fmt.Sprintf(`username="%s"`, c.Username))
	auth = append(auth, fmt.Sprintf(`realm="%s"`, c.Realm))
	auth = append(auth, fmt.Sprintf(`nonce="%s"`, c.Nonce))
	auth = append(auth, fmt.Sprintf(`uri="%s"`, c.URI))
	if c.Opaque != "" {
		auth = append(auth, fmt.Sprintf(`opaque="%s"`, c.Opaque))
	}
	if c.Qop != "" {
		auth = append(auth, fmt.Sprintf(`cnonce="%s"`, c.Cnonce))
		auth = append(auth, fmt.Sprintf("nc=%08x", c.NonceCount))
		auth = append(auth, fmt.Sprintf("qop=%s", c.Qop))
	}
	auth = append(auth, fmt.Sprintf(`response="%s"`, resp))
	if c.Algorithm != "" {
		auth = append(auth, fmt.Sprintf(`algorithm=%s`, c.Algorithm))
	}
	return fmt.Sprintf("Digest %s", strings.Join(auth, ", ")), nil
}
