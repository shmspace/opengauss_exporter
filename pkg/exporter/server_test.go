// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"database/sql"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func Test_parseFingerprint(t *testing.T) {
	type args struct {
		url string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				url: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: "localhost:55432",
		},
		{
			name: "localhost:55432",
			args: args{
				url: "postgres://userDsn:passwordDsn%3D@localhost:55432/?sslmode=disabled",
			},
			want: "localhost:55432",
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				url: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: "127.0.0.1:5432",
		},
		{
			name: "localhost:1234",
			args: args{
				url: "port=1234",
			},

			want: "localhost:1234",
		},
		{
			name: "example:5432",
			args: args{
				url: "host=example",
			},
			want: "example:5432",
		},
		{
			name: "xyz",
			args: args{
				url: "xyz",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFingerprint(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFingerprint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_dbToFloat64(t *testing.T) {
	type args struct {
		t interface{}
	}
	tests := []struct {
		name  string
		args  args
		want  float64
		want1 bool
	}{
		{
			name:  "int64",
			args:  args{t: int64(2)},
			want:  float64(2),
			want1: true,
		},
		{
			name:  "float64",
			args:  args{t: float64(2)},
			want:  float64(2),
			want1: true,
		},
		{
			name:  "time.Time",
			args:  args{t: time.Unix(123456790, 0)},
			want:  float64(123456790),
			want1: true,
		},
		{
			name:  "[]byte",
			args:  args{t: []byte("1234")},
			want:  float64(1234),
			want1: true,
		},
		{
			name:  "string",
			args:  args{t: "232.14"},
			want:  232.14,
			want1: true,
		},
		{
			name:  "bool_true",
			args:  args{t: true},
			want:  1.0,
			want1: true,
		},
		{
			name:  "bool_false",
			args:  args{t: false},
			want:  0.0,
			want1: true,
		},
		// {
		// 	name:"nil",
		// 	args: args{t: nil},
		// 	want: math.NaN(),
		// 	want1: true,
		// },
		// {
		// 	name:"string_NaN",
		// 	args: args{t: "NaN"},
		// 	want: math.NaN(),
		// 	want1: true,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := dbToFloat64(tt.args.t)
			if got != tt.want {
				t.Errorf("dbToFloat64() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dbToFloat64() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_dbToString(t *testing.T) {
	type args struct {
		t interface{}
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{
			name:  "int64",
			args:  args{t: int64(1)},
			want:  "1",
			want1: true,
		},
		{
			name:  "float64",
			args:  args{t: float64(1.1)},
			want:  "1.1",
			want1: true,
		},
		{
			name:  "time.Time",
			args:  args{t: time.Unix(123456790, 0)},
			want:  "123456790",
			want1: true,
		},
		{
			name:  "nil",
			args:  args{t: nil},
			want:  "",
			want1: true,
		},
		{
			name:  "[]byte",
			args:  args{t: []byte("a")},
			want:  "a",
			want1: true,
		},
		{
			name:  "string",
			args:  args{t: "a"},
			want:  "a",
			want1: true,
		},
		{
			name:  "bool_true",
			args:  args{t: true},
			want:  "true",
			want1: true,
		},
		{
			name:  "bool_false",
			args:  args{t: false},
			want:  "false",
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := dbToString(tt.args.t)
			if got != tt.want {
				t.Errorf("dbToString() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dbToString() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_Server(t *testing.T) {
	var (
		db  *sql.DB
		err error
		s   = &Server{
			dsn: "",
			db:  nil,
			labels: prometheus.Labels{
				"server": "localhost:5432",
			},
			master:                 false,
			namespace:              "",
			disableSettingsMetrics: false,
			disableCache:           false,
			lastMapVersion: semver.Version{
				Major: 0,
				Minor: 0,
				Patch: 0,
			},
			queryInstanceMap: defaultMonList,
			mappingMtx:       sync.RWMutex{},
			metricCache:      nil,
			cacheMtx:         sync.Mutex{},
		}
		mock          sqlmock.Sqlmock
		metricName    = "pg_lock"
		queryInstance = defaultMonList[metricName]
	)

	_ = queryInstance.Check()

	t.Run("queryMetric", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname", "mode", "count"}).FromCSVString(`postgres,AccessShareLock,4
omm,RowShareLock,0
postgres,ShareRowExclusiveLock,0
postgres,ShareLock,0
omm,ShareUpdateExclusiveLock,0
omm,ShareLock,0
omm,RowExclusiveLock,0
omm,AccessShareLock,0
omm,ShareRowExclusiveLock,0
postgres,RowExclusiveLock,0
omm,ExclusiveLock,0
postgres,ExclusiveLock,0
postgres,ShareUpdateExclusiveLock,0
omm,AccessExclusiveLock,0
postgres,RowShareLock,0
postgres,AccessExclusiveLock,0`))
		metrics, errs, err := s.queryMetric(metricName, queryInstance)
		assert.NoError(t, err)
		assert.ElementsMatch(t, errs, []error{})
		assert.NotNil(t, metrics)
	})
	t.Run("queryMetric_NoTimeOut", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		queryInstance.Queries[0].Timeout = 0
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname", "mode", "count"}).FromCSVString(`postgres,AccessShareLock,4
omm,RowShareLock,0
postgres,ShareRowExclusiveLock,0
postgres,ShareLock,0
omm,ShareUpdateExclusiveLock,0
omm,ShareLock,0
omm,RowExclusiveLock,0
omm,AccessShareLock,0
omm,ShareRowExclusiveLock,0
postgres,RowExclusiveLock,0
omm,ExclusiveLock,0
postgres,ExclusiveLock,0
postgres,ShareUpdateExclusiveLock,0
omm,AccessExclusiveLock,0
postgres,RowShareLock,0
postgres,AccessExclusiveLock,0`))
		metrics, errs, err := s.queryMetric(metricName, queryInstance)
		assert.NoError(t, err)
		assert.ElementsMatch(t, errs, []error{})
		assert.NotNil(t, metrics)
	})
	t.Run("queryMetric_query_nil", func(t *testing.T) {
		metrics, errs, err := s.queryMetric(metricName, &QueryInstance{})
		assert.NoError(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		assert.ElementsMatch(t, []prometheus.Metric{}, metrics)
	})
	t.Run("queryMetric_timeout", func(t *testing.T) {
		queryInstance.Queries[0].Timeout = 0.1
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillDelayFor(1 * time.Second).WillReturnRows(
			sqlmock.NewRows([]string{"datname", "mode", "count"}).FromCSVString(`postgres,AccessShareLock,4
omm,RowShareLock,0
postgres,ShareRowExclusiveLock,0
postgres,ShareLock,0
omm,ShareUpdateExclusiveLock,0
omm,ShareLock,0
omm,RowExclusiveLock,0
omm,AccessShareLock,0
omm,ShareRowExclusiveLock,0
postgres,RowExclusiveLock,0
omm,ExclusiveLock,0
postgres,ExclusiveLock,0
postgres,ShareUpdateExclusiveLock,0
omm,AccessExclusiveLock,0
postgres,RowShareLock,0
postgres,AccessExclusiveLock,0`))
		metrics, errs, err := s.queryMetric(metricName, queryInstance)
		assert.Error(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		assert.ElementsMatch(t, []prometheus.Metric{}, metrics)
	})
	t.Run("queryMetric_query_err", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("error"))
		metrics, errs, err := s.queryMetric(metricName, queryInstance)
		assert.Error(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		assert.ElementsMatch(t, []prometheus.Metric{}, metrics)
	})
	t.Run("Close", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectClose()
		err := s.Close()
		assert.NoError(t, err)
	})
	t.Run("Close_nil", func(t *testing.T) {
		s.db = nil
		err := s.Close()
		assert.NoError(t, err)
	})
	t.Run("Ping", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectPing()
		err := s.Ping()
		assert.NoError(t, err)
	})
	t.Run("Ping_err", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectPing().WillReturnError(fmt.Errorf("ping error"))
		err := s.Ping()
		assert.Error(t, err)
	})
	t.Run("QueryDatabases", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname"}).FromCSVString(`postgres
omm`))
		r, err := s.QueryDatabases()
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"postgres", "omm"}, r)
	})
}
