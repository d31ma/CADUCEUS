package mailin

func htmlMessage() string {
	return "From: Dan <dan@outside.test>\r\n" +
		"To: Ana <ana@example.com>\r\n" +
		"Subject: Plans\r\n" +
		"Message-ID: <plans-1@outside.test>\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"outer\"\r\n" +
		"\r\n" +
		"--outer\r\n" +
		"Content-Type: multipart/related; boundary=\"inner\"\r\n" +
		"\r\n" +
		"--inner\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>Hello <b>Ana</b></p><img src=\"cid:logo@outside.test\" alt=\"Logo\"><script>alert(1)</script>\r\n" +
		"--inner\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-ID: <logo@outside.test>\r\n" +
		"Content-Disposition: inline; filename=\"logo.png\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"iVBORw0KGgo=\r\n" +
		"--inner--\r\n" +
		"--outer\r\n" +
		"Content-Type: application/pdf; name=\"plan.pdf\"\r\n" +
		"Content-Disposition: attachment; filename=\"plan.pdf\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"JVBERi0xLjQK\r\n" +
		"--outer--\r\n"
}

func textMessage() string {
	return "From: Dan <dan@outside.test>\r\n" +
		"To: Ana <ana@example.com>\r\n" +
		"Subject: Lunch\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Lunch at noon?\r\n"
}
