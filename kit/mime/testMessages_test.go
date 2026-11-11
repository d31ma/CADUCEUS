package mime

import (
	"fmt"
	"strings"
)

type testMessages struct{}

func (testMessages) crlf(text string) []byte {
	return []byte(strings.ReplaceAll(text, "\n", "\r\n"))
}

func (messages testMessages) plain() []byte {
	return messages.crlf(`From: Ana Lima <Ana@Example.COM>
To: Bo <bo@Mesail.Dev>, carl@example.org
Cc: "Dee, D." <dee@EXAMPLE.net>
Subject: Lunch on Friday
Date: Tue, 10 Nov 2026 12:00:00 +0000
Message-ID: <m1.abc@example.com>
In-Reply-To: <m0.xyz@example.com>
References: <r1@example.com> <r2@example.com>
 <r3@example.com><r4@example.com>
Content-Type: text/plain; charset=utf-8

Hello Bo,
see you at noon.
`)
}

func (messages testMessages) alternative() []byte {
	return messages.crlf(`From: ana@example.com
To: bo@example.com
Subject: Report
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="alt-1"

--alt-1
Content-Type: text/plain; charset=us-ascii

Plain report.
--alt-1
Content-Type: text/html; charset=utf-8

<p>Html report.</p>
--alt-1--
`)
}

func (messages testMessages) mixed() []byte {
	return messages.crlf(`From: ana@example.com
To: bo@example.com
Subject: Invoice
Content-Type: multipart/mixed; boundary=mix

--mix
Content-Type: text/plain

See the invoice.
--mix
Content-Type: application/pdf; name="ignored.pdf"
Content-Disposition: attachment; filename="invoice.pdf"
Content-Transfer-Encoding: base64

JVBERi0xLjQK
JSBpbnZvaWNl
--mix--
`)
}

func (messages testMessages) nested() []byte {
	return messages.crlf(`From: ana@example.com
Subject: Nested
Content-Type: multipart/mixed; boundary=outer

--outer
Content-Type: multipart/related; boundary=related

--related
Content-Type: multipart/alternative; boundary=inner

--inner
Content-Type: text/plain; charset=iso-8859-1
Content-Transfer-Encoding: quoted-printable

Caf=E9 menu
--inner
Content-Type: text/html; charset=windows-1252
Content-Transfer-Encoding: quoted-printable

<p>=93Caf=E9=94 <img src=3D"cid:logo"></p>
--inner--
--related
Content-Type: image/png
Content-ID: <logo@example.com>
Content-Transfer-Encoding: base64

iVBORw0KGgo=
--related--
--outer
Content-Type: message/rfc822
Content-Disposition: attachment

From: old@example.com
Subject: Old

Old body.
--outer--
`)
}

func (messages testMessages) fileNames() []byte {
	return messages.crlf(`From: ana@example.com
Content-Type: multipart/mixed; boundary=b

--b
Content-Type: text/plain; charset=utf-8
Content-Disposition: attachment; filename*=UTF-8''na%C3%AFve%20plan.txt

plan
--b
Content-Type: application/octet-stream; name="=?UTF-8?B?w6l0w6kuYmlu?="

data
--b
Content-Type: text/plain; charset=utf-8
Content-Disposition: inline

Second text part.
--b--
`)
}

func (messages testMessages) encodedSubjects() []string {
	return []string{
		"Subject: =?UTF-8?B?Q2Fmw6kgbWVudQ==?=\r\n\r\n",
		"Subject: =?utf-8?Q?Caf=C3=A9_menu?=\r\n\r\n",
		"Subject: =?ISO-8859-1?Q?Caf=E9?= menu\r\n\r\n",
		"Subject: =?windows-1252?Q?Caf=E9?= menu\r\n\r\n",
	}
}

func (messages testMessages) deep(levels int) []byte {
	var builder strings.Builder
	builder.WriteString("From: ana@example.com\r\nContent-Type: multipart/mixed; boundary=b0\r\n\r\n")
	for level := 1; level < levels; level++ {
		fmt.Fprintf(&builder, "--b%d\r\nContent-Type: multipart/mixed; boundary=b%d\r\n\r\n", level-1, level)
	}
	fmt.Fprintf(&builder, "--b%d\r\nContent-Type: text/plain\r\n\r\nbottom\r\n--b%d--\r\n", levels-1, levels-1)
	for level := levels - 2; level >= 0; level-- {
		fmt.Fprintf(&builder, "--b%d--\r\n", level)
	}
	return []byte(builder.String())
}

func (messages testMessages) wide(parts int) []byte {
	var builder strings.Builder
	builder.WriteString("From: ana@example.com\r\nContent-Type: multipart/mixed; boundary=w\r\n\r\n")
	for part := 0; part < parts; part++ {
		fmt.Fprintf(&builder, "--w\r\nContent-Type: application/octet-stream\r\n\r\npart %d\r\n", part)
	}
	builder.WriteString("--w--\r\n")
	return []byte(builder.String())
}

func (messages testMessages) all() [][]byte {
	seeds := [][]byte{messages.plain(), messages.alternative(), messages.mixed(), messages.nested(), messages.fileNames(), messages.deep(3), messages.wide(3)}
	for _, subject := range messages.encodedSubjects() {
		seeds = append(seeds, []byte(subject))
	}
	return append(seeds, []byte(strings.ReplaceAll(string(messages.plain()), "\r\n", "\n")))
}
