package pdftext

import "testing"

func TestParseBBox(t *testing.T) {
	pages := ParseBBox(`<doc><page width="1" height="1"><word xMin="1.5" yMin="2" xMax="11.5" yMax="4">&quot;A&quot;</word></page><page width="1" height="1"><word xMin="0" yMin="0" xMax="2" yMax="2">B</word></page></doc>`)
	if len(pages) != 2 || len(pages[0]) != 1 || pages[0][0].Text != `"A"` || pages[0][0].XC() != 6.5 || pages[0][0].YC() != 3 || pages[1][0].Text != "B" {
		t.Fatalf("%+v", pages)
	}
}
