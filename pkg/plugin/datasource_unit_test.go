package plugin

import (
    "testing"
    "time"
    "github.com/stretchr/testify/require"
    "github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestExpandGrafanaMacros(t *testing.T) {
    from := time.Date(2023, 9, 10, 0, 0, 0, 0, time.UTC)
    to := time.Date(2023, 9, 11, 0, 0, 0, 0, time.UTC)
    tr := backend.TimeRange{From: from, To: to}
    interval := 30 * time.Second

    // $__timeFrom and $__timeTo
    q1 := "select * from users where created_at between $__timeFrom() and $__timeTo()"
    got1 := expandGrafanaMacros(q1, tr, interval, 1000)
    require.Equal(t, "select * from users where created_at between '2023-09-10T00:00:00Z' and '2023-09-11T00:00:00Z'", got1)

    // $__interval_ms
    q2 := "select * from users limit $__interval_ms"
    got2 := expandGrafanaMacros(q2, tr, interval, 1000)
    require.Equal(t, "select * from users limit 30000", got2)

    // $__timeFilter(field)
    q3 := "select * from users where $__timeFilter(created_at)"
    got3 := expandGrafanaMacros(q3, tr, interval, 1000)
    require.Equal(t, "select * from users where created_at >= '2023-09-10T00:00:00Z' and created_at <= '2023-09-11T00:00:00Z'", got3)

    // $timeFilter alias
    q4 := "select * from users where $timeFilter(created_at)"
    got4 := expandGrafanaMacros(q4, tr, interval, 1000)
    require.Equal(t, "select * from users where created_at >= '2023-09-10T00:00:00Z' and created_at <= '2023-09-11T00:00:00Z'", got4)

    // $__timeFilterMs for epoch millis fields
    q5 := "select * from users where $__timeFilterMs(timestamp_epoch)"
    got5 := expandGrafanaMacros(q5, tr, interval, 1000)
    require.Equal(t, "select * from users where timestamp_epoch >= 1694304000000 and timestamp_epoch <= 1694390400000", got5)

    // $timeFilterMs alias
    q6 := "select * from users where $timeFilterMs(timestamp_epoch)"
    got6 := expandGrafanaMacros(q6, tr, interval, 1000)
    require.Equal(t, "select * from users where timestamp_epoch >= 1694304000000 and timestamp_epoch <= 1694390400000", got6)
}


