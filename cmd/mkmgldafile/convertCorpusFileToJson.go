package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/yuui-ro/mglda"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type Data struct {
	Docs []mglda.Document `json:"docs"`
}

func (data *Data) parse(fn string, trainSize int) error {
	fp, error := os.Open(fn)
	if error != nil {
		return error
	}
	defer fp.Close()

	reader := bufio.NewReader(fp)

	var docs = []mglda.Document{}
	counter := 0

	var line string
	line, reader_err := reader.ReadString('\n')
	for {
		line = strings.TrimSpace(line)
		if reader_err != nil && len(line) == 0 {
			break
		}

		d := mglda.Document{}

		if trainSize < 0 || counter < trainSize {
			d.State = mglda.Active
		} else {
			d.State = mglda.Holdout
		}

		for _, s := range strings.Split(line, "|") {
			sentence := mglda.Sentense{}
			w, err := ReadInts(s)
			if err != nil {
				panic(err)
			}
			sentence.Words = w
			d.Sentenses = append(d.Sentenses, sentence)
		}

		docs = append(docs, d)
		counter++
		line, reader_err = reader.ReadString('\n')
	}

	data.Docs = docs

	return nil
}

// ReadInts reads whitespace-separated ints from a string.
// If there's an error,
// it returns the ints successfully read so far as well as the error value.
func ReadInts(txt string) ([]int, error) {
	r := strings.NewReader(txt)
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	var result []int
	for scanner.Scan() {
		x, err := strconv.Atoi(scanner.Text())
		if err != nil {
			return result, err
		}
		result = append(result, x)
	}
	return result, nil
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	corpusFile := flag.String("corpus_file", "corpus", "Corpus file")
	trainSize := flag.Int("train_size", -1, "Number of documents for training") // if trainSize less than zero, then all documents are used for training
	outputFile := flag.String("output_file", "output.json", "Output file in Json format")
	flag.Parse()

	data := &Data{}
	err := data.parse(*corpusFile, *trainSize)
	check(err)

	fmt.Printf("Read %d lines.\n", len(data.Docs))

	b, err := json.Marshal(data)
	if err != nil {
		check(err)
	}

	ioutil.WriteFile(*outputFile, b, 0644)

}
