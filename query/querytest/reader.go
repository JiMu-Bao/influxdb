package querytest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/influxdata/flux"
	"github.com/influxdata/flux/execute"
	"github.com/influxdata/flux/execute/executetest"
	"github.com/influxdata/influxdb/query/stdlib/influxdata/influxdb"
	"github.com/influxdata/influxdb/storage/reads"
)

type errTransformation struct {
	err error
}

func (t *errTransformation) RetractTable(id execute.DatasetID, key flux.GroupKey) error {
	return nil
}

func (t *errTransformation) Process(id execute.DatasetID, tbl flux.Table) error {
	return nil
}

func (t *errTransformation) UpdateWatermark(id execute.DatasetID, mark execute.Time) error {
	return nil
}

func (t *errTransformation) UpdateProcessingTime(id execute.DatasetID, time execute.Time) error {
	return nil
}

func (t *errTransformation) Finish(id execute.DatasetID, err error) {
	t.err = err
}

func ReaderTestHelper(
	t *testing.T,
	start execute.Time,
	stop execute.Time,
	addTransformations func(d execute.Dataset, c execute.TableBuilderCache, s execute.Source) error,
	data []*Table,
	want []*executetest.Table,
	wantErr bool,
) {
	t.Helper()

	if start > stop {
		panic("invalid bounds, start > stop")
	}

	for _, tbl := range data {
		if err := tbl.Check(); err != nil {
			panic(err)
		}
	}

	bounds := execute.Bounds{
		Start: start,
		Stop:  stop,
	}
	now := stop

	store, err := NewStore(data, bounds)
	if err != nil {
		t.Fatal(err)
	}

	id := executetest.RandomDatasetID()
	d := executetest.NewDataset(id)
	c := execute.NewTableBuilderCache(executetest.UnlimitedAllocator)
	c.SetTriggerSpec(flux.DefaultTrigger)
	s := influxdb.NewSource(
		id,
		reads.NewReader(store),
		influxdb.ReadSpec{PointsLimit: math.MaxInt64},
		bounds,
		execute.Window{
			Period: execute.Duration(stop - start),
			Every:  execute.Duration(stop - start),
		},
		now)

	err = addTransformations(d, c, s)
	if err != nil {
		t.Fatal(err)
	}

	errT := &errTransformation{}
	s.AddTransformation(errT)
	s.Run(context.Background())
	if errT.err != nil {
		if !wantErr {
			t.Fatal(err)
		} else {
			return
		}
	} else if wantErr {
		t.Fatal(fmt.Errorf("wanted error, got none"))
	}

	got, err := executetest.TablesFromCache(c)
	if err != nil {
		t.Fatal(err)
	}

	executetest.NormalizeTables(got)
	executetest.NormalizeTables(want)

	sort.Sort(executetest.SortedTables(got))
	sort.Sort(executetest.SortedTables(want))

	if !cmp.Equal(want, got) {
		t.Errorf("unexpected tables -want/+got\n%s", cmp.Diff(want, got))
	}
}
