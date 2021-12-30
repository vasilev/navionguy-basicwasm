package cli

import (
	"errors"
	"testing"

	"github.com/navionguy/basicwasm/mocks"
	"github.com/navionguy/basicwasm/object"
)

func TestExecCommand(t *testing.T) {
	tests := []struct {
		inp string
		exp []string
	}{
		{inp: ""},
		{inp: `10 PRINT X`},
		{inp: "RESTORE X", exp: []string{"undefined line number X"}},
		{inp: "CHAIN", exp: []string{"Type mismatch", "OK"}},
		{inp: `PRINT "HELLO"`, exp: []string{"HELLO", "", "OK"}},
	}

	for _, tt := range tests {
		trm := mocks.MockTerm{}
		mocks.InitMockTerm(&trm)
		trm.ExpMsg = &mocks.Expector{}
		if len(tt.exp) > 0 {
			trm.ExpMsg.Exp = tt.exp
		}
		env := object.NewTermEnvironment(trm)
		execCommand(tt.inp, env)
		if len(tt.exp) > 0 {
			if trm.ExpMsg.Failed {
				t.Fatalf("%s didn't expect that!", tt.inp)
			}

			if len(trm.ExpMsg.Exp) != 0 {
				t.Fatalf("%s expected %s but didn't get it", tt.inp, trm.ExpMsg.Exp[0])

			}

		}
	}
}

func Test_GiveError(t *testing.T) {
	tests := []struct {
		inp string
		exp []string
	}{
		{inp: "Syntax Error", exp: []string{"Syntax Error", "OK"}},
	}

	for _, tt := range tests {
		var trm mocks.MockTerm
		mocks.InitMockTerm(&trm)
		trm.ExpMsg = &mocks.Expector{}
		trm.ExpMsg.Exp = tt.exp
		env := object.NewTermEnvironment(trm)
		terr := errors.New(tt.inp)
		giveError(terr.Error(), env)
		if trm.ExpMsg.Failed {
			t.Fatalf("GiveError didn't")
		}
	}

}
