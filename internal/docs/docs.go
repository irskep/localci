package docs

import _ "embed"

//go:embed generated/localci.1
var manPage string

//go:embed generated/localci.txt
var plainText string

func ManPage() string {
	return manPage
}

func PlainText() string {
	return plainText
}
