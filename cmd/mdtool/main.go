package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Matir/mdtool/converter"
	"github.com/Matir/mdtool/server"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var (
	cssPath     string
	noHighlight bool
	noMermaid   bool
	listenAddr  string
	onlyMD      bool
	watch       bool
	frontmatter string
	addTitle    string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mdtool [flags] <inpath> [outpath]",
		Short: "mdtool renders Markdown files into HTML",
		Long:  "mdtool is a tool for rendering Markdown files as HTML. It supports CommonMark, GitHub Flavored Markdown (GFM), syntax highlighting, Mermaid diagrams, custom CSS inlining, and YAML front matter handling.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Legacy/Default Batch mode
			if len(args) < 1 {
				return cmd.Help()
			}
			c := getConverter()
			inPath := args[0]
			var outPath string
			if len(args) > 1 {
				outPath = args[1]
			}
			if watch {
				return watchBatch(c, inPath, outPath)
			}
			return runBatch(c, inPath, outPath)
		},
	}

	var convertCmd = &cobra.Command{
		Use:   "convert <inpath> [outpath]",
		Short: "Batch convert Markdown files or directories to HTML",
		Long:  "Convert a single Markdown file or recursively convert a directory of Markdown files into self-contained HTML files.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getConverter()
			inPath := args[0]
			var outPath string
			if len(args) > 1 {
				outPath = args[1]
			}
			if watch {
				return watchBatch(c, inPath, outPath)
			}
			return runBatch(c, inPath, outPath)
		},
	}

	var serveCmd = &cobra.Command{
		Use:   "serve [directory]",
		Short: "Serve Markdown files as HTML via a local web server",
		Long:  "Run a local web server that dynamically converts and serves Markdown files as HTML on demand, with directory navigation and optional live-reload watching.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			c := getConverter()
			c.Watch = watch
			s := server.New(dir, listenAddr, onlyMD, c)
			s.Watch = watch
			return s.Serve()
		},
	}

	rootCmd.PersistentFlags().StringVar(&cssPath, "css", "", "Path to custom CSS file to inline into rendered HTML")
	rootCmd.PersistentFlags().BoolVar(&noHighlight, "no-highlight", false, "Disable syntax highlighting in code blocks")
	rootCmd.PersistentFlags().BoolVar(&noMermaid, "no-mermaid", false, "Disable Mermaid.js diagram rendering")
	rootCmd.PersistentFlags().BoolVarP(&watch, "watch", "w", false, "Watch input for changes and automatically re-convert or auto-reload")
	rootCmd.PersistentFlags().StringVar(&frontmatter, "frontmatter", "auto", "Front matter handling before conversion: auto (remove if valid YAML), remove (always remove if delimited by ---), include (keep intact)")
	rootCmd.PersistentFlags().StringVar(&addTitle, "addtitle", "auto", "Title insertion mode: auto (add h1 if missing), on/true (always add h1), off/false (never add h1)")

	serveCmd.Flags().StringVarP(&listenAddr, "listen", "l", "127.0.0.1:7768", "Listen address and port for web server")
	serveCmd.Flags().BoolVar(&onlyMD, "only-md", false, "Only serve and display Markdown (.md) files in directory listing")

	rootCmd.AddCommand(convertCmd, serveCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getConverter() *converter.Converter {
	var css string
	if cssPath != "" {
		data, err := os.ReadFile(cssPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading CSS file: %v\n", err)
			os.Exit(1)
		}
		css = string(data)
	}
	c := converter.New(css, !noHighlight, !noMermaid)
	c.Watch = watch
	c.Frontmatter = frontmatter
	c.AddTitle = addTitle
	return c
}

func watchBatch(c *converter.Converter, inPath, outPath string) error {
	// Initial conversion
	if err := runBatch(c, inPath, outPath); err != nil {
		log.Printf("Initial conversion error: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Watch input directory/file
	info, err := os.Stat(inPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		filepath.Walk(inPath, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				watcher.Add(path)
			}
			return nil
		})
	} else {
		watcher.Add(filepath.Dir(inPath))
	}

	fmt.Printf("Watching %s for changes...\n", inPath)

	// Debounce timer
	var timer *time.Timer
	const delay = 100 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if strings.HasSuffix(event.Name, ".md") {
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(delay, func() {
						fmt.Printf("Change detected in %s, re-converting...\n", event.Name)
						if err := runBatch(c, inPath, outPath); err != nil {
							log.Printf("Re-conversion error: %v", err)
						}
					})
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func runBatch(c *converter.Converter, inPath, outPath string) error {
	info, err := os.Stat(inPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return convertDir(c, inPath, outPath)
	}
	return convertFile(c, inPath, outPath)
}

func convertFile(c *converter.Converter, inPath, outPath string) error {
	if outPath == "" {
		outPath = strings.TrimSuffix(inPath, filepath.Ext(inPath)) + ".html"
	} else {
		outInfo, err := os.Stat(outPath)
		if err == nil && outInfo.IsDir() {
			outPath = filepath.Join(outPath, strings.TrimSuffix(filepath.Base(inPath), filepath.Ext(inPath))+".html")
		}
	}

	return doConvert(c, inPath, outPath)
}

func convertDir(c *converter.Converter, inPath, outPath string) error {
	if outPath != "" {
		if err := os.MkdirAll(outPath, 0755); err != nil {
			return err
		}
	}

	return filepath.Walk(inPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		var target string
		rel, _ := filepath.Rel(inPath, path)
		if outPath == "" {
			target = strings.TrimSuffix(path, filepath.Ext(path)) + ".html"
		} else {
			target = filepath.Join(outPath, strings.TrimSuffix(rel, filepath.Ext(rel))+".html")
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
		}

		fmt.Printf("Converting %s -> %s\n", path, target)
		return doConvert(c, path, target)
	})
}

func doConvert(c *converter.Converter, in, out string) error {
	fIn, err := os.Open(in)
	if err != nil {
		return err
	}
	defer fIn.Close()

	fOut, err := os.Create(out)
	if err != nil {
		return err
	}
	defer fOut.Close()

	return c.Convert(fIn, fOut)
}
