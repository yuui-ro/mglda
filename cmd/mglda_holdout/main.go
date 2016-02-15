package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/yuui-ro/mglda"
	"io/ioutil"
	"math"
	"os"
)

type Configuration struct {
	GlobalK        int     `json:"global_k"`
	LocalK         int     `json:"local_k"`
	Gamma          float64 `json:"gamma"`
	GlobalAlpha    float64 `json:"global_alpha"`
	LocalAlpha     float64 `json:"local_alpha"`
	GlobalAlphaMix float64 `json:"global_alpha_mix"`
	LocalAlphaMix  float64 `jons:"local_alpha_mix"`
	GlobalBeta     float64 `json:"global_beta"`
	LocalBeta      float64 `json:"local_beta"`
	T              int     `json:"t"`
	W              int     `json:"w"`
}

func (d *Configuration) parse(fn string) error {
	bt, err := ioutil.ReadFile(fn)
	if err != nil {
		return err
	}
	err = json.Unmarshal(bt, d)
	return err
}

type Data struct {
	Docs []mglda.Document `json:"docs"`
}

func (d *Data) parse(fn string) error {
	bt, err := ioutil.ReadFile(fn)
	if err != nil {
		return err
	}
	err = json.Unmarshal(bt, d)
	return err
}

func sumOfArrayFloat64(array *[]float64) float64 {
	sum := 0.0
	for _, e := range *array {
		sum += e
	}
	return sum
}

func sumOfArrayInt(array *[]int) int {
	sum := 0
	for _, e := range *array {
		sum += e
	}
	return sum
}

func main() {
	dataFile := flag.String("data", "data.json", "Data file in json")
	trainBurnin := *flag.Int("train_burnin", 3000, "Number of burnin iterations for training data")
	testBurnin := *flag.Int("test_burnin", 3000, "Number of burnin iterations for each test document")
	sampleSpace := *flag.Int("sample_space", 500, "Number of iterations for evaluating the harmonic mean for each holdout document")
	confFile := flag.String("config", "conf.json", "Configration file for the settings of parameters")
	loglikeFile := flag.String("loglikefile", "loglikefile", "Output file for loglikelihood of holdout documents")
	docnumFile := flag.String("docnumfile", "docnumfile", "Output file for number of words of holdout documents")
	perplexityFile := flag.String("perplexityfile", "perplexityfile", "Output file for perplexity of holdout documents")

	flag.Parse()

	fmt.Println("dataFile:", *dataFile, "configFile", *confFile)
	conf := &Configuration{}
	if err := conf.parse(*confFile); err != nil {
		panic(err)
	}

	data := Data{}
	if err := data.parse(*dataFile); err != nil {
		panic(err)
	}

	fmt.Println("dataFile:", *dataFile, "configFile", *confFile)

	docs := data.Docs
	m := mglda.NewMGLDA(conf.GlobalK, conf.LocalK, conf.Gamma,
		conf.GlobalAlpha, conf.LocalAlpha,
		conf.GlobalAlphaMix, conf.LocalAlphaMix,
		conf.GlobalBeta, conf.LocalBeta,
		conf.T, conf.W, &docs)

	out := os.Stdout
	wt := bufio.NewWriter(out)
	defer wt.Flush()

	_, dochmloglik, numWords := mglda.EvaluateHoldout(m, trainBurnin,
		testBurnin, sampleSpace, wt)

	holdoutLoglik := sumOfArrayFloat64(&dochmloglik)
	holdoutNumWords := sumOfArrayInt(&numWords)
	holdoutPerplexity := math.Exp(-1.0 * holdoutLoglik / float64(holdoutNumWords))

	ioutil.WriteFile(*docnumFile, []byte(fmt.Sprintf("%d", holdoutNumWords)), 0x644)
	ioutil.WriteFile(*loglikeFile, []byte(fmt.Sprintf("%f", holdoutLoglik)), 0x644)
	ioutil.WriteFile(*perplexityFile, []byte(fmt.Sprintf("%f", holdoutPerplexity)), 0x644)

}
