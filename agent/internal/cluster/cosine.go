package cluster

import "math"

// Cosine returns the cosine similarity of two vectors, in [-1, 1].
//
// Mismatched lengths or a zero vector yield 0: no similarity rather than a
// panic, because a provider returning a short vector must not kill the run.
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Centroid returns the component-wise mean of the vectors.
//
// Vectors of the wrong width are skipped rather than truncated, so one bad
// response cannot corrupt a cluster's centre.
func Centroid(vectors [][]float32) []float32 {
	width := 0
	for _, v := range vectors {
		if len(v) > 0 {
			width = len(v)
			break
		}
	}
	if width == 0 {
		return nil
	}

	sum := make([]float64, width)
	n := 0
	for _, v := range vectors {
		if len(v) != width {
			continue
		}
		for i, x := range v {
			sum[i] += float64(x)
		}
		n++
	}
	if n == 0 {
		return nil
	}

	out := make([]float32, width)
	for i, s := range sum {
		out[i] = float32(s / float64(n))
	}
	return out
}
