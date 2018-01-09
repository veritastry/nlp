package nlp

import (
	"bytes"
	"math"
	"math/rand"
	"testing"

	"github.com/james-bowman/nlp/measures/pairwise"
	"github.com/james-bowman/sparse"
	"gonum.org/v1/gonum/mat"
)

func TestTruncatedSVDFitTransform(t *testing.T) {
	var tests = []struct {
		m      int
		n      int
		input  []float64
		k      int
		r      int
		c      int
		result []float64
	}{
		{
			m: 6, n: 4,
			input: []float64{
				1, 3, 5, 2,
				8, 1, 0, 0,
				2, 1, 0, 1,
				0, 0, 0, 0,
				0, 0, 0, 1,
				0, 1, 0, 0,
			},
			k: 2,
			r: 2, c: 4,
			result: []float64{
				-8.090, -2.212, -1.695, -0.955,
				1.888, -2.524, -4.649, -1.930,
			},
		},
	}

	for _, test := range tests {
		transformer := NewTruncatedSVD(test.k)
		input := mat.NewDense(test.m, test.n, test.input)
		expResult := mat.NewDense(test.r, test.c, test.result)

		result, err := transformer.FitTransform(input)

		if err != nil {
			t.Errorf("Failed Truncated SVD transform caused by %v", err)
		}

		if !mat.EqualApprox(expResult, result, 0.01) {
			t.Logf("Expected matrix: \n%v\n but found: \n%v\n",
				mat.Formatted(expResult),
				mat.Formatted(result))
			t.Fail()
		}

		result2, err := transformer.Transform(input)

		if err != nil {
			t.Errorf("Failed Truncated SVD transform caused by %v", err)
		}

		if !mat.EqualApprox(result, result2, 0.001) {
			t.Logf("First matrix: \n%v\n but second matrix: \n%v\n",
				mat.Formatted(result),
				mat.Formatted(result2))
			t.Fail()
		}
	}
}

func TestTruncatedSVDSaveLoad(t *testing.T) {
	var transforms = []struct {
		wanted *TruncatedSVD
	}{
		{
			wanted: &TruncatedSVD{
				Components: mat.NewDense(4, 2, []float64{
					1, 5,
					3, 2,
					9, 0,
					8, 4,
				}),
				K: 2,
			},
		},
	}

	for ti, test := range transforms {
		t.Logf("**** TestTruncatedSVDSaveLoad - Test Run %d.\n", ti+1)

		buf := new(bytes.Buffer)
		if err := test.wanted.Save(buf); err != nil {
			t.Errorf("Error encoding: %v\n", err)
			continue
		}

		var b TruncatedSVD
		if err := b.Load(buf); err != nil {
			t.Errorf("Error unencoding: %v\n", err)
			continue
		}

		if !mat.Equal(test.wanted.Components, b.Components) {
			t.Logf("Components mismatch: Wanted %v but got %v\n", mat.Formatted(test.wanted.Components), mat.Formatted(b.Components))
			t.Fail()
		}
		if test.wanted.K != b.K {
			t.Logf("K value mismatch: Wanted %d but got %d\n", test.wanted.K, b.K)
			t.Fail()
		}
	}
}

func TestSignRandomProjections(t *testing.T) {
	tests := []struct {
		rows int
		cols int
		bits int
	}{
		{rows: 100, cols: 1000, bits: 1024},
		{rows: 100, cols: 1000, bits: 256},
	}

	for ti, test := range tests {
		// Given an input matrix and a query matching one column
		matrix := mat.NewDense(test.rows, test.cols, nil)
		for i := 0; i < test.rows; i++ {
			for j := 0; j < test.cols; j++ {
				matrix.Set(i, j, rand.Float64())
			}
		}

		query := matrix.ColView(0)

		// When transformed using sign random projections
		transformer := NewSignRandomProjection(test.bits)
		reducedDimMatrix, err := transformer.FitTransform(matrix)
		if err != nil {
			t.Errorf("Failed to transform matrix because %v\n", err)
		}
		m := reducedDimMatrix.(*sparse.Binary)

		reducedDimQuery, err := transformer.Transform(query)
		if err != nil {
			t.Errorf("Failed to transform query because %v\n", err)
		}
		q := reducedDimQuery.(*sparse.BinaryVec)

		var culmDiff float64
		for i := 0; i < test.cols; i++ {
			angSim := pairwise.AngularSimilarity(query, matrix.ColView(i))
			lshSim := pairwise.HammingSimilarity(q, m.ColView(i))

			if i == 0 {
				if math.Abs(angSim-lshSim) >= 0.0000001 {
					t.Errorf("Test %d: Expected matching similarity but found %.10f (Ang) and %.10f (LSH)\n", ti, angSim, lshSim)
				}
			}

			diff := math.Abs(lshSim-angSim) / angSim
			culmDiff += diff
		}
		avgDiff := culmDiff / float64(test.cols)

		// Then output matrix should be of specified length,
		// matching column should still have similarity of ~1.0 and
		// avg difference betwen angular and hamming similarities should
		// be less than 0.03
		r, c := m.Dims()
		if r != test.bits || c != test.cols {
			t.Errorf("Test %d: Expected output matrix to be %dx%d but was %dx%d\n", ti, test.bits, test.cols, r, c)
		}
		if avgDiff >= 0.03 {
			t.Errorf("Test %d: Expected difference between vector spaces %f but was %f\n", ti, 0.3, avgDiff)
		}
	}
}
