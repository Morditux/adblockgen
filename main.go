package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
)

var sourcePath = flag.String("s", "", "source path")
var destinationPath = flag.String("d", "", "destination path")
var checkValidHosts = flag.Bool("c", false, "save only valid hosts")
var destinationIP = flag.String("dip", "0.0.0.0", "destination ip")
var invalidFile = flag.String("i", "", "invalid file")

type StringWriter struct {
	buffer []string
}

func NewStringWriter() *StringWriter {
	return &StringWriter{
		buffer: make([]string, 0),
	}
}

func (s *StringWriter) Write(p []byte) (int, error) {
	s.buffer = append(s.buffer, string(p))
	return len(p), nil
}

func (s *StringWriter) WriteString(data string) (int, error) {
	s.buffer = append(s.buffer, data)
	return len(data), nil
}

func (s *StringWriter) Sort() {
	slices.Sort(s.buffer)
}

func (s *StringWriter) Len() int {
	return len(s.buffer)
}

func (s *StringWriter) Strings() []string {
	return s.buffer
}

func saveHostFile(hosts []string, filename string) error {
	log.Println("saving hosts file")
	slices.Sort(hosts)
	out, err := os.Create(filename)
	writer := bufio.NewWriter(out)
	if err != nil {
		return err
	}
	defer out.Close()
	for _, host := range hosts {
		if host != "" {
			s := fmt.Sprintf("address=/%s/%s\n", host, *destinationIP)
			_, err = writer.WriteString(s)
			if err != nil {
				return err
			}
		}
	}
	writer.Flush()
	return nil
}

func loadSources(filename string) ([]string, error) {
	urls := make([]string, 0)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			urls = append(urls, line)
		}
	}
	return urls, nil
}

func saveInvalidUrls(urls []string, filename string) error {
	log.Println("saving invalid hosts file")
	slices.Sort(urls)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, url := range urls {
		_, err = fmt.Fprintln(writer, url)
		if err != nil {
			return err
		}
	}
	writer.Flush()
	return nil
}

func loadInvalidUrls(filename string) (map[string]any, error) {
	invalids := make(map[string]any)
	file, err := os.Open(filename)
	if err != nil {
		return invalids, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			invalids[line] = nil
		}
	}
	return invalids, nil
}

func mergeInvalidsUrl(invalids map[string]any, newInvalids []string) []string {
	tmp := make(map[string]any)

	for host, _ := range invalids {
		tmp[host] = nil
	}

	for _, host := range newInvalids {
		tmp[host] = nil
	}
	r := make([]string, 0)
	for host, _ := range tmp {
		r = append(r, host)
	}
	return r
}

func main() {
	var err error
	var inputFile string
	var outputFile string
	var validOnly bool
	var invalidHosts map[string]any

	flag.Parse()
	if *sourcePath == "" {
		fmt.Println("Missing source file (-s filename)")
		os.Exit(-1)
	}
	inputFile = *sourcePath
	if *destinationPath == "" {
		fmt.Println("Missing destination file (-d filename)")
		os.Exit(-1)
	}
	validOnly = *checkValidHosts
	outputFile = *destinationPath

	if validOnly {
		if *invalidFile == "" {
			fmt.Println("Missing invalid host file (-i filename)")
			os.Exit(-1)
		}
		invalidHosts, err = loadInvalidUrls(*invalidFile)
		if err != nil {
			invalidHosts = make(map[string]any)
		}
	}

	urls, err := loadSources(inputFile)
	if err != nil {
		log.Fatal(err)
	}
	downloader := NewDownloader(urls, invalidHosts)
	downloader.Run()
	hosts := downloader.GetHosts()
	if validOnly {
		hc := NewHostChecker(hosts, 500)
		hc.Start()
		hosts = hc.Valids()
		invalids := hc.Invalids()
		if len(invalids) > 0 {
			invalids = mergeInvalidsUrl(invalidHosts, invalids)
			err = saveInvalidUrls(invalids, *invalidFile)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	// Create output file
	err = saveHostFile(hosts, outputFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d lines writen.", len(hosts))
	log.Println("done...")
}
