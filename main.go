package main

import (
	"flag"
	"html/template"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

var sourceDir string
var outputDir string
var tmplDir string

func main() {
	flag.StringVar(&sourceDir, "src", "", "folder containing articles")
	flag.StringVar(&outputDir, "out", "", "destination root folder for articles")
	flag.StringVar(&tmplDir, "tpl", "", "template file for articles")
	flag.Parse()

	if sourceDir == "" || outputDir == "" || tmplDir == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	renderJobs := make(chan renderJob, 0)
	renderRes := make(chan renderRes, 0)
	articles := make(chan Res[string], 0)

	go runArticleHTMLRenderer(renderJobs, renderRes)
	go findArticlesPaths(sourceDir, articles)

	go func() {
		for res := range articles {
			if res.Err != nil {
				log.Print(res.Err)
				continue
			}

			name := path.Base(res.Ok)
			chunks := strings.Split(name, ".")
			if len(chunks) != 3 {
				log.Printf("wrong filename format for file %s", name)
				continue
			}
			outPath := path.Join(outputDir, chunks[1], "index.html")

			renderJobs <- renderJob{
				inPath:  res.Ok,
				outPath: outPath,
			}
		}
		close(renderJobs)
	}()

	for res := range renderRes {
		if res.err != nil {
			log.Printf("failed to render %s: %v", res.path, res.err)
		} else {
			log.Printf("successfully rendered %s", res.path)
		}
	}

	log.Print("finished")
}

func renderArticleHTML(dst io.Writer, src io.Reader) error {
	panic("TODO")
}

type renderJob struct {
	inPath  string
	outPath string
}

type renderRes struct {
	path string
	err  error
}

func runArticleHTMLRenderer(jobs <-chan renderJob, res chan<- renderRes) {
	tmplPath := path.Join(tmplDir, "article.html")
	tmpl := template.Must(template.ParseFiles(tmplPath))
	type tmplData struct {
		Title     string
		CreatedAt string
		Content   template.HTML
	}

	wg := new(sync.WaitGroup)

	for job := range jobs {
		job := job
		wg.Add(1)

		go func() {
			defer wg.Done()

			b, err := os.ReadFile(job.inPath)
			if err != nil {
				res <- renderRes{
					path: job.inPath,
					err:  err,
				}
				return
			}

			parser := parser.New()
			renderer := html.NewRenderer(html.RendererOptions{})

			doc := parser.Parse(b)
			body := markdown.Render(doc, renderer)

			err = os.MkdirAll(path.Dir(job.outPath), 0777)
			if err != nil {
				res <- renderRes{
					path: job.inPath,
					err:  err,
				}
				return
			}

			dst, err := os.Create(job.outPath)
			if err != nil {
				res <- renderRes{
					path: job.inPath,
					err:  err,
				}
				return
			}

			err = tmpl.Execute(dst, tmplData{
				Title:     "title",
				CreatedAt: "23/07/23",
				Content:   template.HTML(body),
			})
			if err != nil {
				res <- renderRes{
					path: job.inPath,
					err:  err,
				}
				return
			}

			res <- renderRes{
				path: job.outPath,
			}
		}()
	}

	wg.Wait()
	close(res)
}

func findArticlesPaths(dir string, res chan<- Res[string]) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		res <- Err[string](err)
	}

	for _, e := range entries {
		p := path.Join(dir, e.Name())
		if strings.HasSuffix(p, ".md") {
			res <- Ok[string](p)
		}
	}

	close(res)
}

type Res[T any] struct {
	Ok  T
	Err error
}

func Ok[T any](v T) Res[T] {
	return Res[T]{Ok: v}
}

func Err[T any](err error) Res[T] {
	return Res[T]{Err: err}
}
