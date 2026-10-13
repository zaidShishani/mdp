package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

type content struct {
	Title string
	Name  string
	Body  template.HTML
}

const (
	defaultTemplate = `<!DOCTYPE html>
<html>
<head>
<meta http-equiv="content-type" content="text/html; charset=utf-8">
<title>{{ .Title }}</title>
</head>
<body>
<h1>{{ .Name }}</h1>
{{ .Body }}
</body>
</html>
`
)

func main() {
	filename := flag.String("file", "", "Markdown File Preview")
	skipPreview := flag.Bool("s", false, "Skip auto preview")
	tFname := flag.String("t", "", "Alternate template name")
	flag.Parse()

	var in = os.Stdin
	name := "stdin"
	if *filename != "" {
		f, err := os.Open(*filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		in, name = f, *filename
	}

	if err := run(name, in, *tFname, os.Stdout, *skipPreview); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(name string, in io.Reader, tFname string, out io.Writer, skipPreview bool) error {
	input, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	htmlData, err := parseContent(name, input, tFname)
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp("", "mdp*.html")
	if err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}

	outName := temp.Name()
	fmt.Fprintln(out, outName)

	if err := saveHTML(outName, htmlData); err != nil {
		return err
	}

	if skipPreview {
		return nil
	}

	defer os.Remove(outName)

	return preview(outName)
}

func parseContent(name string, input []byte, tFname string) ([]byte, error) {
	unsafe := blackfriday.Run(input)
	body := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	body = bytes.TrimRight(bytes.ReplaceAll(body, []byte("\n\n"), []byte("\n")), "\n")

	t, err := template.New("mdp").Parse(defaultTemplate)
	if err != nil {
		return nil, err
	}

	if tFname == "" {
		tFname = os.Getenv("FILE_TEMPLATE")
	}

	if tFname != "" {
		t, err = template.ParseFiles(tFname)
		if err != nil {
			return nil, err
		}
	}

	displayName := strings.TrimSuffix(name, ".md")
	c := content{
		Title: "Markdown Preview Tool",
		Name:  displayName,
		Body:  template.HTML(body),
	}

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, c); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func saveHTML(outName string, data []byte) error {
	return os.WriteFile(outName, data, 0644)
}

func preview(fname string) error {
	cName := ""
	cParams := []string{}

	switch runtime.GOOS {
	case "linux":
		cName = "xdg-open"
	case "windows":
		cName = "cmd.exe"
		cParams = []string{"/C", "start"}
	case "darwin":
		cName = "open"
	default:
		return fmt.Errorf("OS not supported")
	}

	cParams = append(cParams, fname)
	cPath, err := exec.LookPath(cName)
	if err != nil {
		return err
	}

	err = exec.Command(cPath, cParams...).Run()
	time.Sleep(2 * time.Second)
	return err
}
