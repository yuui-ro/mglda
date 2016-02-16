package mglda

import (
	"bufio"
	"fmt"
	"github.com/golang/glog"
	"github.com/gonum/floats"
	"github.com/skelterjohn/go.matrix"
	"math"
	"math/rand"
)

const (
	topicLimit  = 20
	globalTopic = "gl"
	localTopic  = "loc"
)

type DocumentState uint

const (
	Active DocumentState = iota
	Frozen
	Holdout
)

type Document struct {
	Sentenses []Sentense `json:"sentenses"`
	State     DocumentState
}

type Sentense struct {
	Words []int `json:"words"`
}

func (d *Document) NumberOfWords() int {
	nn := 0
	for _, s := range (*d).Sentenses {
		nn += len(s.Words)
	}
	return nn
}

type MGLDA struct {
	GlobalK        int
	LocalK         int
	Gamma          float64
	GlobalAlpha    float64
	LocalAlpha     float64
	GlobalAlphaMix float64
	LocalAlphaMix  float64
	GlobalBeta     float64
	LocalBeta      float64
	Docs           *[]Document
	T              int
	W              int
	Inflation      float64
	Vdsn           [][][]int
	Rdsn           [][][]string
	Zdsn           [][][]int
	Nglzw          *matrix.DenseMatrix
	Nglz           *matrix.DenseMatrix
	Ndsv           [][][]float64
	Nds            [][]float64
	Ndvgl          [][]float64
	Ndv            [][]float64
	Ndglz          *matrix.DenseMatrix
	Ndgl           *matrix.DenseMatrix
	Nloczw         *matrix.DenseMatrix
	Nlocz          *matrix.DenseMatrix
	Ndvloc         [][]float64
	Ndvlocz        [][][]float64
}

func (m *MGLDA) LogLikelihood() float64 {
	ll := float64(0)
	for i := 0; i < m.GlobalK; i++ {
		ss := 0
		for j := 0; j < m.W; j++ {
			Nzw := int(m.Nglzw.Get(i, j))
			for n := 0; n < Nzw; n++ {
				ll += math.Log((float64(n) + m.GlobalBeta) / (float64(ss) + float64(m.W)*m.GlobalBeta))
				ss++
			}
		}
	}

	for i := 0; i < m.LocalK; i++ {
		ss := 0
		for j := 0; j < m.W; j++ {
			Nzw := int(m.Nloczw.Get(i, j))
			for n := 0; n < Nzw; n++ {
				ll += math.Log((float64(n) + m.LocalBeta) / (float64(ss) + float64(m.W)*m.LocalBeta))
				ss++
			}
		}
	}

	return ll
}

// Inference runs a go routine for each doc.
func (m *MGLDA) Inference() {
	for d, doc := range *m.Docs {
		if doc.State != Active {
			continue
		}
		for s, sent := range doc.Sentenses {
			for w, wd := range sent.Words {
				v := m.Vdsn[d][s][w]
				r := m.Rdsn[d][s][w]
				z := m.Zdsn[d][s][w]

				if r == globalTopic {
					m.Nglzw.Set(z, wd, m.Nglzw.Get(z, wd)-1)
					m.Nglz.Set(z, 0, m.Nglz.Get(z, 0)-1)
					m.Ndvgl[d][s+v] -= 1
					m.Ndglz.Set(d, z, m.Ndglz.Get(d, z)-1)
					m.Ndgl.Set(d, 0, m.Ndgl.Get(d, 0)-1)
				} else {
					m.Nloczw.Set(z, wd, m.Nloczw.Get(z, wd)-1)
					m.Nlocz.Set(z, 0, m.Nlocz.Get(z, 0)-1)
					m.Ndvloc[d][s+v] -= 1
					m.Ndvlocz[d][s+v][z] -= 1
				}
				m.Ndsv[d][s][v] -= 1
				m.Nds[d][s] -= 1
				m.Ndv[d][s+v] -= 1

				pvrz := []float64{}
				newVs := []int{}
				newRs := []string{}
				newZs := []int{}
				for vt := 0; vt < m.T; vt++ {
					for zt := 0; zt < m.GlobalK; zt++ {
						newVs = append(newVs, vt)
						newRs = append(newRs, globalTopic)
						newZs = append(newZs, zt)
						term1 := (m.Nglzw.Get(zt, wd) + m.GlobalBeta) / (m.Nglz.Get(zt, 0) + float64(m.W)*m.GlobalBeta)
						term2 := (m.Ndsv[d][s][vt] + m.Gamma) / (m.Nds[d][s] + float64(m.T)*m.Gamma)
						term3 := (m.Ndvgl[d][s+vt] + m.GlobalAlphaMix) / (m.Ndv[d][s+vt] + m.GlobalAlphaMix + m.LocalAlphaMix)
						term4 := (m.Ndglz.Get(d, zt) + m.GlobalAlpha) / (m.Ndgl.Get(d, 0) + float64(m.GlobalK)*m.GlobalAlpha)
						pvrz = append(pvrz, term1*term2*term3*term4)

					}
					for zt := 0; zt < m.LocalK; zt++ {
						newVs = append(newVs, vt)
						newRs = append(newRs, localTopic)
						newZs = append(newZs, zt)
						term1 := (m.Nloczw.Get(zt, wd) + m.LocalBeta) / (m.Nlocz.Get(zt, 0) + float64(m.W)*m.LocalBeta)
						term2 := (m.Ndsv[d][s][vt] + m.Gamma) / (m.Nds[d][s] + float64(m.T)*m.Gamma)
						term3 := (m.Ndvloc[d][s+vt] + m.LocalAlphaMix) / (m.Ndv[d][s+vt] + m.GlobalAlphaMix + m.LocalAlphaMix)
						term4 := (m.Ndvlocz[d][s+vt][zt] + m.LocalAlpha) / (m.Ndvloc[d][s+vt] + float64(m.LocalK)*m.LocalAlpha)
						pvrz = append(pvrz, term1*term2*term3*term4)
					}
				}

				// sampling from multinomial distribution
				var randIdx int
				var sum float64
				for _, item := range pvrz {
					sum += item
				}

				threshold := rand.Float64()
				partialSum := 0.0
				for i := 0; i < len(pvrz); i++ {
					partialSum += pvrz[i] / sum
					if partialSum >= threshold {
						randIdx = i
						break
					}
				}
				newV := newVs[randIdx]
				newR := newRs[randIdx]
				newZ := newZs[randIdx]
				// update
				if newR == globalTopic {
					m.Nglzw.Set(newZ, wd, m.Nglzw.Get(newZ, wd)+1)
					m.Nglz.Set(newZ, 0, m.Nglz.Get(newZ, 0)+1)
					m.Ndvgl[d][s+newV] += 1
					m.Ndglz.Set(d, newZ, m.Ndglz.Get(d, newZ)+1)
					m.Ndgl.Set(d, 0, m.Ndgl.Get(d, 0)+1)
				} else {
					m.Nloczw.Set(newZ, wd, m.Nloczw.Get(newZ, wd)+1)
					m.Nlocz.Set(newZ, 0, m.Nlocz.Get(newZ, 0)+1)
					m.Ndvloc[d][s+newV] += 1
					m.Ndvlocz[d][s+newV][newZ] += 1
				}
				m.Ndsv[d][s][newV] += 1
				m.Nds[d][s] += 1
				m.Ndv[d][s+newV] += 1

				m.Vdsn[d][s][w] = newV
				m.Rdsn[d][s][w] = newR
				m.Zdsn[d][s][w] = newZ
			}
		}
	}
}

// WordDist returns a topic word distribution
func (m *MGLDA) WordDist() (*matrix.DenseMatrix, *matrix.DenseMatrix) {
	newNglz := m.Nglz.Copy()
	if err := newNglz.AddDense(matrix.Ones(newNglz.Rows(), newNglz.Cols())); err != nil {
		panic(err)
	}

	newNlocz := m.Nlocz.Copy()
	if err := newNlocz.AddDense(matrix.Ones(m.Nlocz.Rows(), m.Nlocz.Cols())); err != nil {
		panic(err)
	}

	newNglzw := m.Nglzw.Copy()
	for i := 0; i < newNglzw.Rows(); i++ {
		newNglzw.ScaleRow(i, float64(1)/newNglz.Get(i, 0))
	}

	newNloczw := m.Nloczw.Copy()
	for i := 0; i < newNloczw.Rows(); i++ {
		newNloczw.ScaleRow(i, float64(1)/newNlocz.Get(i, 0))
	}
	return newNglzw, newNloczw
}

func NewMGLDA(globalK, localK int, gamma, globalAlpha, localAlpha,
	globalAlphaMix, localAlphaMix, globalBeta, localBeta float64,
	t, w int, docs *[]Document) *MGLDA {
	docLen := len(*docs)
	inflation := float64(0)
	m := MGLDA{
		GlobalK:        globalK,
		LocalK:         localK,
		Gamma:          gamma,
		GlobalAlpha:    globalAlpha,
		LocalAlpha:     localAlpha,
		GlobalAlphaMix: globalAlphaMix,
		LocalAlphaMix:  localAlphaMix,
		GlobalBeta:     globalBeta,
		LocalBeta:      localBeta,
		Docs:           docs,
		T:              t,
		W:              w,
		Inflation:      inflation,
		Nglzw:          matrix.Zeros(globalK, w),
		Nglz:           matrix.Zeros(globalK, 1),
		Ndglz:          matrix.Zeros(docLen, globalK),
		Ndgl:           matrix.Zeros(docLen, 1),
		Nloczw:         matrix.Zeros(localK, w),
		Nlocz:          matrix.Zeros(localK, 1),
	}

	glog.Info("random fitting MGLDA")
	for _, doc := range *docs {
		vd := [][]int{}
		rd := [][]string{}
		zd := [][]int{}
		ndsvd := [][]float64{}
		ndsd := []float64{}
		initialValue := matrix.Numbers(len(doc.Sentenses)+t, 1, inflation).Array()
		m.Ndvloc = append(m.Ndvloc, initialValue)
		m.Ndvlocz = append(m.Ndvlocz, matrix.Numbers(len(doc.Sentenses)+t, localK, inflation).Arrays())
		m.Ndv = append(m.Ndvloc, initialValue)
		m.Ndvgl = append(m.Ndvgl, initialValue)

		for _, sts := range doc.Sentenses {
			vs := []int{}
			rs := []string{}
			zs := []int{}
			for _ = range sts.Words {
				vs = append(vs, rand.Intn(t))

				tp := rand.Intn(2)
				var r string
				var z int
				if tp == 0 {
					r = globalTopic
					z = rand.Intn(globalK)
				} else {
					r = localTopic
					z = rand.Intn(localK)
				}
				rs = append(rs, r)
				zs = append(zs, z)
			}
			vd = append(vd, vs)
			rd = append(rd, rs)
			zd = append(zd, zs)

			ndsvs := []float64{}
			for i := 0; i < t; i++ {
				ndsvs = append(ndsvs, inflation)
			}
			ndsvd = append(ndsvd, ndsvs)
			ndsd = append(ndsd, inflation)
		}
		m.Vdsn = append(m.Vdsn, vd)
		m.Rdsn = append(m.Rdsn, rd)
		m.Zdsn = append(m.Zdsn, zd)
		m.Ndsv = append(m.Ndsv, ndsvd)
		m.Nds = append(m.Nds, ndsd)
	}

	glog.Info("initializing")
	for d, doc := range *docs {
		if doc.State == Holdout {
			continue
		}
		for s, sts := range doc.Sentenses {
			for w, wd := range sts.Words {
				v := m.Vdsn[d][s][w]
				r := m.Rdsn[d][s][w]
				z := m.Zdsn[d][s][w]
				if r == globalTopic {
					m.Nglzw.Set(z, wd, m.Nglzw.Get(z, wd)+1)
					m.Nglz.Set(z, 0, m.Nglz.Get(z, 0)+1)
					m.Ndvgl[d][s+v] += 1
					m.Ndglz.Set(d, z, m.Ndglz.Get(d, z)+1)
					m.Ndgl.Set(d, 0, m.Ndgl.Get(d, 0)+1)
				} else {
					m.Nloczw.Set(z, wd, m.Nloczw.Get(z, wd)+1)
					m.Nlocz.Set(z, 0, m.Nlocz.Get(z, 0)+1)
					m.Ndvloc[d][s+v] += 1
					m.Ndvlocz[d][s+v][z] += 1
				}
				m.Ndsv[d][s][v] += 1
				m.Nds[d][s] += 1
				m.Ndv[d][s+v] += 1
			}
		}
	}

	return &m
}

func (m *MGLDA) loadDocument(d int) {
	for s, sts := range (*m.Docs)[d].Sentenses {
		for w, wd := range sts.Words {
			v := m.Vdsn[d][s][w]
			r := m.Rdsn[d][s][w]
			z := m.Zdsn[d][s][w]
			if r == globalTopic {
				m.Nglzw.Set(z, wd, m.Nglzw.Get(z, wd)+1)
				m.Nglz.Set(z, 0, m.Nglz.Get(z, 0)+1)
				m.Ndvgl[d][s+v] += 1
				m.Ndglz.Set(d, z, m.Ndglz.Get(d, z)+1)
				m.Ndgl.Set(d, 0, m.Ndgl.Get(d, 0)+1)
			} else {
				m.Nloczw.Set(z, wd, m.Nloczw.Get(z, wd)+1)
				m.Nlocz.Set(z, 0, m.Nlocz.Get(z, 0)+1)
				m.Ndvloc[d][s+v] += 1
				m.Ndvlocz[d][s+v][z] += 1
			}
			m.Ndsv[d][s][v] += 1
			m.Nds[d][s] += 1
			m.Ndv[d][s+v] += 1
		}
	}
}

func (m *MGLDA) unloadDocument(d int) {
	for s, sts := range (*m.Docs)[d].Sentenses {
		for w, wd := range sts.Words {
			v := m.Vdsn[d][s][w]
			r := m.Rdsn[d][s][w]
			z := m.Zdsn[d][s][w]
			if r == globalTopic {
				m.Nglzw.Set(z, wd, m.Nglzw.Get(z, wd)-1)
				m.Nglz.Set(z, 0, m.Nglz.Get(z, 0)-1)
				m.Ndvgl[d][s+v] -= 1
				m.Ndglz.Set(d, z, m.Ndglz.Get(d, z)-1)
				m.Ndgl.Set(d, 0, m.Ndgl.Get(d, 0)-1)
			} else {
				m.Nloczw.Set(z, wd, m.Nloczw.Get(z, wd)-1)
				m.Nlocz.Set(z, 0, m.Nlocz.Get(z, 0)-1)
				m.Ndvloc[d][s+v] -= 1
				m.Ndvlocz[d][s+v][z] -= 1
			}
			m.Ndsv[d][s][v] -= 1
			m.Nds[d][s] -= 1
			m.Ndv[d][s+v] -= 1
		}
	}
}

func GetWordTopicDist(m *MGLDA, vocabulary []string, wt *bufio.Writer) {
	zGlCount := make([]int, m.GlobalK)
	zLocCount := make([]int, m.LocalK)
	wordGlCount := []map[int]int{}
	wordLocCount := []map[int]int{}
	for i := 0; i < m.GlobalK; i++ {
		wordGlCount = append(wordGlCount, map[int]int{})
	}
	for i := 0; i < m.LocalK; i++ {
		wordLocCount = append(wordLocCount, map[int]int{})
	}

	glog.Info("Get words distribution")
	for d, doc := range *m.Docs {
		for s, sent := range doc.Sentenses {
			for w, wd := range sent.Words {
				r := m.Rdsn[d][s][w]
				z := m.Zdsn[d][s][w]
				if r == globalTopic {
					zGlCount[z] += 1
					wordGlCount[z][wd] += 1
				} else {
					zLocCount[z] += 1
					wordLocCount[z][wd] += 1
				}
			}
		}
	}
	glog.Info("Done dist")
	phiGl, phiLoc := m.WordDist()
	for i := 0; i < m.GlobalK; i++ {
		header := fmt.Sprintf("-- global topic: %d (%d words)\n", i, zGlCount[i])
		wt.WriteString(header)
		glog.Info(header)
		rows := phiGl.RowCopy(i)
		idx := []int{}
		for j := 0; j < len(rows); j++ {
			idx = append(idx, j)
		}
		floats.Argsort(rows, idx)
		for j := len(idx) - 1; j > len(idx)-topicLimit; j-- {
			w := idx[j]
			tp := fmt.Sprintf("%s: %f (%d)\n",
				vocabulary[w], phiGl.Get(i, w),
				wordGlCount[i][w])
			wt.WriteString(tp)
			glog.Info(tp)
		}
	}
	for i := 0; i < m.LocalK; i++ {
		header := fmt.Sprintf("-- local topic: %d (%d words)\n", i, zLocCount[i])
		wt.WriteString(header)
		glog.Info(header)
		rows := phiLoc.RowCopy(i)
		idx := []int{}
		for j := 0; j < len(rows); j++ {
			idx = append(idx, j)
		}
		floats.Argsort(rows, idx)
		for j := len(idx) - 1; j > len(idx)-topicLimit; j-- {
			w := idx[j]
			tp := fmt.Sprintf("%s: %f (%d)\n",
				vocabulary[w], phiLoc.Get(i, w),
				wordLocCount[i][w])
			wt.WriteString(tp)
			glog.Info(tp)
		}
	}

}

func Learning(m *MGLDA, iteration int, vocabulary []string, wt *bufio.Writer) {
	for i := 0; i < iteration; i++ {
		wt.WriteString(fmt.Sprintf("==== %d-th inference ====\n", i))
		glog.Info(fmt.Sprintf("==== %d-th inference ====\n", i))
		m.Inference()
		glog.Info("inference completed")
		GetWordTopicDist(m, vocabulary, wt)
	}
}

func logAddition(logaa float64, logbb float64) float64 {
	var rr float64
	if logaa > logbb {
		rr = logaa + math.Log(1.0+math.Exp(logbb-logaa))
	} else {
		rr = logbb + math.Log(1.0+math.Exp(logaa-logbb))
	}
	return rr
}

func EvaluateHoldout(m *MGLDA, trainBurnin int, testBurnin int, sampleSpace int, wt *bufio.Writer) ([]int, []float64, []int) {
	var testDocNo []int
	var dochmloglik []float64
	var numWords []int

	wt.WriteString("Running burnin....\n")
	for i := 0; i < trainBurnin; i++ {
		wt.WriteString(fmt.Sprintf("iterate %d.\n", i))
		wt.Flush()
		if i%20 == 0 {
			wt.WriteString(fmt.Sprintf("    loglikelihood=%f.\n", m.LogLikelihood()))
		}
		m.Inference()
	}
	beforeLoglik := m.LogLikelihood()

	// freeze the current active documents
	wt.WriteString("Freeze the active documents ...\n")
	wt.Flush()
	for i := 0; i < len(*m.Docs); i++ {
		if (*m.Docs)[i].State == Active {
			(*m.Docs)[i].State = Frozen
		}
	}

	wt.WriteString("Evaluate holdout documents ...\n")
	for dno := 0; dno < len(*m.Docs); dno++ {
		ptr := &(*m.Docs)[dno]
		if ptr.State == Frozen {
			continue
		}

		if ptr.State == Holdout {
			wt.WriteString(fmt.Sprintf("Evaluate document %d.\n", dno))
			wt.Flush()
			ptr.State = Active
			hmloglik := -100.0
			m.loadDocument(dno)
			for i := 0; i < testBurnin+sampleSpace; i++ {
				m.Inference()
				if i >= testBurnin {
					afterLoglik := m.LogLikelihood()
					hmloglik = logAddition(hmloglik, beforeLoglik-afterLoglik)
				}
			}
			hmloglik = math.Log(float64(sampleSpace)) - hmloglik
			m.unloadDocument(dno)
			ptr.State = Holdout

			testDocNo = append(testDocNo, dno)
			dochmloglik = append(dochmloglik, hmloglik)
			numWords = append(numWords, ptr.NumberOfWords())
		}
	}

	// activate the frozen documents
	wt.WriteString("Active the frozen documents...\n")
	for i := 0; i < len(*m.Docs); i++ {
		if (*m.Docs)[i].State == Frozen {
			(*m.Docs)[i].State = Active
		}
	}

	return testDocNo, dochmloglik, numWords
}
