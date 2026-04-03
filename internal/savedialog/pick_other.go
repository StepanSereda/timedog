//go:build !darwin && !linux

package savedialog

// PickSaveReportJSONL is not implemented on this platform.
func PickSaveReportJSONL(suggested string) (string, error) {
	_ = suggested
	return "", ErrUnavailable
}
