package mimeparse

import (
	"reflect"
	"testing"
)

// The expectations were produced by mailparser's simpleParser, which the
// JavaScript SMTP servers used.
func TestMatchesMailparser(t *testing.T) {
	cases := []struct {
		name, raw, subject, from string
		hasSubject               bool
		to, cc                   []string
		text, html               string
	}{
		{"plain", "From: \"Ann\" <Ann@Example.TEST>\r\nTo: bob@d.test, \"C, D\" <cd@d.test>\r\nCc: Group: e@d.test, f@d.test;\r\nSubject: =?UTF-8?B?w6ljaG8=?= test\r\n\r\nHello\r\nWorld\r\n",
			"écho test", "Ann@Example.TEST", true, []string{"bob@d.test", "cd@d.test"}, nil, "Hello\nWorld\n", ""},
		{"alt", "From: a@b.test\r\nTo: x@y.test\r\nSubject: alt\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"B\"\r\n\r\n--B\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\ncaf=E9 =\r\nnoir\r\n--B\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\nPHA+Y2Fmw6k8L3A+\r\n--B--\r\n",
			"alt", "a@b.test", true, []string{"x@y.test"}, nil, "café noir", "<p>café</p>"},
		{"nosubject", "From: a@b.test\r\n\r\nbody\r\n", "", "a@b.test", false, nil, nil, "body\n", ""},
		{"attach", "From: a@b.test\r\nSubject: att\r\nContent-Type: multipart/mixed; boundary=X\r\n\r\n--X\r\nContent-Type: text/plain\r\n\r\nmain text\r\n--X\r\nContent-Type: text/plain; name=a.txt\r\nContent-Disposition: attachment; filename=a.txt\r\n\r\nattached\r\n--X--\r\n",
			"att", "a@b.test", true, nil, nil, "main text", ""},
	}
	for _, c := range cases {
		message, err := Parse([]byte(c.raw))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if message.Subject != c.subject || message.HasSubject != c.hasSubject || len(message.From) == 0 || message.From[0] != c.from {
			t.Fatalf("%s: subject %q (%v) from %v", c.name, message.Subject, message.HasSubject, message.From)
		}
		if !reflect.DeepEqual(message.To, c.to) || !reflect.DeepEqual(message.Cc, c.cc) {
			t.Fatalf("%s: to %#v cc %#v", c.name, message.To, message.Cc)
		}
		if message.Text != c.text || message.HTML != c.html {
			t.Fatalf("%s: text %q html %q", c.name, message.Text, message.HTML)
		}
	}
}

func TestHTMLOnlyMessageGetsText(t *testing.T) {
	message, _ := Parse([]byte("Content-Type: text/html\r\n\r\n<p>Hello&nbsp;<b>there</b></p><p>Bye</p>"))
	if message.Text != "Hello there\nBye" {
		t.Fatalf("text = %q", message.Text)
	}
}
