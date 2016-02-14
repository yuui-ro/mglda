package main

import (
	"bufio"
	"encoding/json"
	"flag"
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
	scanner := bufio.NewScanner(reader)

	var docs = []mglda.Document{}

	counter := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		d := mglda.Document{}

		if counter < trainSize {
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
	trainSize := flag.Int("train_size", -1, "Number of documents for training")
	outputFile := flag.String("output_file", "output.json", "Output file in Json format")
	flag.Parse()

	data := &Data{}
	err := data.parse(*corpusFile, *trainSize)
	check(err)

	b, err := json.Marshal(data)
	if err != nil {
		check(err)
	}

	ioutil.WriteFile(*outputFile, b, 0644)

}
