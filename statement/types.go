package statement

type Statement string

const (
	Base64Ratio float64 = 1.5
)

// Estimating the buffer size for converting n statement array to, may not use
// this actually, the encoding speed is slower than memory growth I think
func EstimateJsonBufferCap(nstmt []Statement) int {
	s := 0
	for _, i := range nstmt {
		added := float64(len(i)) * Base64Ratio
		s += int(added)
	}
	return s
}
