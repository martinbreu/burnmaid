#!/bin/bash
from="$1" # 2024/01/01 12:00:00
to="$2"

gnuplot -persist <<-EOFMarker
     set grid
     set xdata time
     set timefmt "%Y/%m/%d %H:%M:%S"
     set xrange ["$from":"$to"]
     set yrange [0:1350]
     set ytics 50
     set terminal png size 1280, 960
     set output "plot.png"
     plot './../data/burnmaid.log' using 1:4 with linespoints linetype 7 linewidth 3 title 'Sensor', \
          './../data/burnmaid.log' using 1:3 with linespoints linetype 2 linewidth 1 title 'Wunsch'
EOFMarker