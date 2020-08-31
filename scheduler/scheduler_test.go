package scheduler

import (
	"testing"

	"github.com/go-test/deep"
)

func TestParseCPUList(t *testing.T) {
	tests := []struct {
		input string
		want  *CPU
		err   bool
	}{
		{
			input: "0-23,48-71",
			want: &CPU{
				physicalCoreMin: 0,
				physicalCoreMax: 23,
				logicalCoreMin:  48,
				logicalCoreMax:  71,
				corePairs: []CorePair{
					{0, 48},
					{1, 49},
					{2, 50},
					{3, 51},
					{4, 52},
					{5, 53},
					{6, 54},
					{7, 55},
					{8, 56},
					{9, 57},
					{10, 58},
					{11, 59},
					{12, 60},
					{13, 61},
					{14, 62},
					{15, 63},
					{16, 64},
					{17, 65},
					{18, 66},
					{19, 67},
					{20, 68},
					{21, 69},
					{22, 70},
					{23, 71},
				},
			},
			err: false,
		},
		{
			input: "24-47,72-95",
			want: &CPU{
				physicalCoreMin: 24,
				physicalCoreMax: 47,
				logicalCoreMin:  72,
				logicalCoreMax:  95,
				corePairs: []CorePair{
					{24, 72},
					{25, 73},
					{26, 74},
					{27, 75},
					{28, 76},
					{29, 77},
					{30, 78},
					{31, 79},
					{32, 80},
					{33, 81},
					{34, 82},
					{35, 83},
					{36, 84},
					{37, 85},
					{38, 86},
					{39, 87},
					{40, 88},
					{41, 89},
					{42, 90},
					{43, 91},
					{44, 92},
					{45, 93},
					{46, 94},
					{47, 95},
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
		got, err := ParseCPUList(test.input)
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

func TestScheduler_PopCorePair(t *testing.T) {
	cpu1, _ := ParseCPUList("0-3,8-11")
	cpu2, _ := ParseCPUList("4-7,12-15")
	scheduler := NewScheduler([]CPU{*cpu1, *cpu2})

	tests := []struct {
		input int
		want  []CorePair
		err   bool
	}{
		{
			input: 2,
			want: []CorePair{
				{
					PhysicalCore: 0,
					LogicalCore:  8,
				},
				{
					PhysicalCore: 1,
					LogicalCore:  9,
				},
			},
			err: false,
		},
		{
			input: 2,
			want: []CorePair{
				{
					PhysicalCore: 2,
					LogicalCore:  10,
				},
				{
					PhysicalCore: 3,
					LogicalCore:  11,
				},
			},
			err: false,
		},
		{
			input: 4,
			want: []CorePair{
				{
					PhysicalCore: 4,
					LogicalCore:  12,
				},
				{
					PhysicalCore: 5,
					LogicalCore:  13,
				},
				{
					PhysicalCore: 6,
					LogicalCore:  14,
				},
				{
					PhysicalCore: 7,
					LogicalCore:  15,
				},
			},
			err: false,
		},
		{
			input: 2,
			want:  nil,
			err:   true,
		},
	}
	for _, test := range tests {
		got, err := scheduler.PopCorePair(test.input)
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

func TestScheduler_PushCorePair(t *testing.T) {
	cpu1, _ := ParseCPUList("0-3,8-11")
	cpu2, _ := ParseCPUList("4-7,12-15")
	scheduler := NewScheduler([]CPU{*cpu1, *cpu2})

	for i := 0; i < 3; i++ {
		_, _ = scheduler.PopCorePair(2)
	}

	tests := []struct {
		input []CorePair
		want  *Scheduler
		err   bool
	}{
		{
			input: []CorePair{
				{
					PhysicalCore: 2,
					LogicalCore:  10,
				},
				{
					PhysicalCore: 3,
					LogicalCore:  11,
				},
			},
			want: &Scheduler{
				mutex: scheduler.mutex,
				cpus: []CPU{
					{
						physicalCoreMin: 0,
						physicalCoreMax: 3,
						logicalCoreMin:  8,
						logicalCoreMax:  11,
						corePairs: []CorePair{
							{3, 11},
							{2, 10},
						},
					},
					{
						physicalCoreMin: 4,
						physicalCoreMax: 7,
						logicalCoreMin:  9,
						logicalCoreMax:  15,
						corePairs: []CorePair{
							{6, 14},
							{7, 15},
						},
					},
				},
			},
			err: false,
		},
		{
			input: []CorePair{
				{
					PhysicalCore: 4,
					LogicalCore:  12,
				},
			},
			want: &Scheduler{
				mutex: scheduler.mutex,
				cpus: []CPU{
					{
						physicalCoreMin: 0,
						physicalCoreMax: 3,
						logicalCoreMin:  8,
						logicalCoreMax:  11,
						corePairs: []CorePair{
							{3, 11},
							{2, 10},
						},
					},
					{
						physicalCoreMin: 4,
						physicalCoreMax: 7,
						logicalCoreMin:  9,
						logicalCoreMax:  15,
						corePairs: []CorePair{
							{4, 12},
							{6, 14},
							{7, 15},
						},
					},
				},
			},
			err: false,
		},
		{
			input: []CorePair{
				{16, 20},
			},
			want: nil,
			err:  true,
		},
	}
	for _, test := range tests {
		err := scheduler.PushCorePair(test.input)
		if !test.err && err != nil {
			t.Fatalf("should not be error for %+v but: %+v", test.input, err)
		}
		if test.err && err == nil {
			t.Fatalf("should be error for %+v but not:", test.input)
		}
		for i := 0; test.want != nil && i < len(scheduler.cpus); i++ {
			if diff := deep.Equal(test.want.cpus[i].corePairs, scheduler.cpus[i].corePairs); len(diff) != 0 {
				t.Fatalf("want %+v, but %+v, diff %q:", test.want, scheduler, diff)
			}
		}
	}
}
