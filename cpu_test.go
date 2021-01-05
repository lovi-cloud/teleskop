package main

import (
	"testing"

	"github.com/go-test/deep"
	pb "github.com/lovi-cloud/satelit/api/satelit_datastore"
)

func TestParseNodeList(t *testing.T) {
	tests := []struct {
		input string
		want  *pb.NumaNode
		err   bool
	}{
		{
			input: "0-23,48-71",
			want: &pb.NumaNode{
				PhysicalCoreMin: 0,
				PhysicalCoreMax: 23,
				LogicalCoreMin:  48,
				LogicalCoreMax:  71,
				Pairs: []*pb.CorePair{
					{PhysicalCore: 0, LogicalCore: 48},
					{PhysicalCore: 1, LogicalCore: 49},
					{PhysicalCore: 2, LogicalCore: 50},
					{PhysicalCore: 3, LogicalCore: 51},
					{PhysicalCore: 4, LogicalCore: 52},
					{PhysicalCore: 5, LogicalCore: 53},
					{PhysicalCore: 6, LogicalCore: 54},
					{PhysicalCore: 7, LogicalCore: 55},
					{PhysicalCore: 8, LogicalCore: 56},
					{PhysicalCore: 9, LogicalCore: 57},
					{PhysicalCore: 10, LogicalCore: 58},
					{PhysicalCore: 11, LogicalCore: 59},
					{PhysicalCore: 12, LogicalCore: 60},
					{PhysicalCore: 13, LogicalCore: 61},
					{PhysicalCore: 14, LogicalCore: 62},
					{PhysicalCore: 15, LogicalCore: 63},
					{PhysicalCore: 16, LogicalCore: 64},
					{PhysicalCore: 17, LogicalCore: 65},
					{PhysicalCore: 18, LogicalCore: 66},
					{PhysicalCore: 19, LogicalCore: 67},
					{PhysicalCore: 20, LogicalCore: 68},
					{PhysicalCore: 21, LogicalCore: 69},
					{PhysicalCore: 22, LogicalCore: 70},
					{PhysicalCore: 23, LogicalCore: 71},
				},
			},
			err: false,
		},
		{
			input: "24-47,72-95",
			want: &pb.NumaNode{
				PhysicalCoreMin: 24,
				PhysicalCoreMax: 47,
				LogicalCoreMin:  72,
				LogicalCoreMax:  95,
				Pairs: []*pb.CorePair{
					{PhysicalCore: 24, LogicalCore: 72},
					{PhysicalCore: 25, LogicalCore: 73},
					{PhysicalCore: 26, LogicalCore: 74},
					{PhysicalCore: 27, LogicalCore: 75},
					{PhysicalCore: 28, LogicalCore: 76},
					{PhysicalCore: 29, LogicalCore: 77},
					{PhysicalCore: 30, LogicalCore: 78},
					{PhysicalCore: 31, LogicalCore: 79},
					{PhysicalCore: 32, LogicalCore: 80},
					{PhysicalCore: 33, LogicalCore: 81},
					{PhysicalCore: 34, LogicalCore: 82},
					{PhysicalCore: 35, LogicalCore: 83},
					{PhysicalCore: 36, LogicalCore: 84},
					{PhysicalCore: 37, LogicalCore: 85},
					{PhysicalCore: 38, LogicalCore: 86},
					{PhysicalCore: 39, LogicalCore: 87},
					{PhysicalCore: 40, LogicalCore: 88},
					{PhysicalCore: 41, LogicalCore: 89},
					{PhysicalCore: 42, LogicalCore: 90},
					{PhysicalCore: 43, LogicalCore: 91},
					{PhysicalCore: 44, LogicalCore: 92},
					{PhysicalCore: 45, LogicalCore: 93},
					{PhysicalCore: 46, LogicalCore: 94},
					{PhysicalCore: 47, LogicalCore: 95},
				},
			},
			err: false,
		},
		{
			input: "0-23",
			want:  nil,
			err:   true,
		},
		{
			input: "24-47",
			want:  nil,
			err:   true,
		},
	}
	for _, test := range tests {
		got, err := ParseNodeList(test.input)
		if !test.err && err != nil {
			t.Fatalf("should not be error for %+v but: %+v", test.input, err)
		}
		if test.err && err == nil {
			t.Fatalf("should be error for %+v but not:", test.input)
		}
		if diff := deep.Equal(test.want, got); len(diff) != 0 {
			t.Fatalf("want %q, but %q, diff %q:", test.want, got, diff)
		}
	}
}
