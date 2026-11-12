package db

import "testing"

func TestGenerateMask(t *testing.T) {
	for in, want := range map[string]string{
		"UA443220010000026000000009976":        "UA44-322001-0000-9976",
		"ua44 3220 0100 0002 6000 0000 0997 6": "UA44-322001-0000-9976",
		"414949******0172":                     "",
		"BN1234560041":                         "BN-0041",
		"BN12345":                              "",
		"":                                     "",
	} {
		if got := GenerateMask(in); got != want {
			t.Errorf("GenerateMask(%q)=%q want %q", in, got, want)
		}
	}
}

func TestIBANMasksAutoAndCustom(t *testing.T) {
	s, _, _ := newS(t)
	iban := "UA443220010000026000000009976"
	s.CreateUser(User{ID: "u1", Name: "Док"})
	s.AddUserAlias("u1", "Док")
	if err := s.LinkAccount("u1", "Док", iban, ""); err != nil {
		t.Fatal(err)
	}
	m, err := s.IBANMasks()
	if err != nil || m[iban].Masked != "UA44-322001-0000-9976" || m[iban].Custom {
		t.Fatalf("auto mask: %+v %v", m, err)
	}
	if got, _ := s.SetIBANMask(iban, "  Мой   ФОП "); got.Masked != "Мой ФОП" || !got.Custom {
		t.Fatalf("custom: %+v", got)
	}
	if m, _ = s.IBANMasks(); m[iban].Masked != "Мой ФОП" || !m[iban].Custom {
		t.Fatalf("custom must survive reads: %+v", m[iban])
	}
	if got, _ := s.SetIBANMask(iban, ""); got.Custom || got.Masked != "UA44-322001-0000-9976" {
		t.Fatalf("reset to auto: %+v", got)
	}
}
