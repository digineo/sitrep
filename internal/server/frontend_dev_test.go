//go:build dev

package server

func testAssets() *assets {
	a, _ := newAssets()
	return a
}
