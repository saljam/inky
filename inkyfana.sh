#!/bin/bash
set -e

tempdat=$(mktemp)
temppng=$(mktemp)
trap "rm -f $tempdat $temppng" EXIT

query="${1:-"sum by ('phase')(power_watts{instance=~'em.*'})"}"
promserver="${promserver:-http://n:9090}"
end=$(date -u +%s)
start=$(($end - (24 * 60 * 60)))
step=1m

echo $query

curl \
	--data-urlencode "end=$end" \
	--data-urlencode "start=$start" \
	--data-urlencode "step=$step" \
	--data-urlencode "query=$query" \
	"${promserver}/api/v1/query_range" \
	| jq -r '.data.result[] | (.metric | to_entries[0].key +":"+ to_entries[0].value) as $m | (.values[] | {t:.[0]|tostring, v: .[1], m: $m})' | jq  -r -s 'reduce .[] as $v ({}; .[$v.t][$v.m]=$v.v) | to_entries[] | .key +" "+ .value["phase:a"] + " "+.value["phase:b"] + " " +.value["phase:c"]' > $tempdat
	# for line plot: | jq -r '.data.result[] | (.metric | "\n\n" + to_entries[0].key +":"+ to_entries[0].value), (.values[] | (.[0]|tostring) +" "+ .[1])'

gnuplot <<EOF
set xdata time
set timefmt "%s"
set title "استهلاك الطاقة آخر ٢٤ ساعة"
set ylabel "الطاقة (كيلوواط)"
set xlabel " "
set terminal unknown
plot '$tempdat' i 0 u 1:(\$2/1000)
set terminal pngcairo size 800, 480 background rgb '0xD9F2FF'
set output '${temppng}'
set xrange [GPVAL_DATA_X_MIN:GPVAL_DATA_X_MAX]
plot \
	'$tempdat' u 1:((\$2+\$3+\$4)/1000) w filledcurves x1 title 'ا' lc rgb '0xF55022', \
	'' u 1:((\$3+\$4)/1000) w filledcurves x1 title 'ٮ' lc rgb '0xFFFF44', \
	'' u 1:((\$4)/1000) w filledcurves x1 title 'حـ' lc rgb '0x1B2EC6'
EOF

sudo $HOME/bin/inky $temppng
