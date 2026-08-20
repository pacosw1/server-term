package collector

import (
	"bytes"
	"context"
	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Collector struct{ SSH config.SSHConfig }

func (c Collector) Collect(ctx context.Context, server config.Server) metrics.Sample {
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, c.SSH.CommandTimeout)
	defer cancel()
	var cmd *exec.Cmd
	script := linuxScript
	if server.IsLocal() {
		if runtime.GOOS == "darwin" {
			script = darwinScript
		}
		cmd = exec.CommandContext(ctx, "sh", "-s")
	} else {
		args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=" + strconv.Itoa(max(1, int(c.SSH.ConnectTimeout.Seconds())))}
		if c.SSH.StrictHostKeyChecking != nil && !*c.SSH.StrictHostKeyChecking {
			args = append(args, "-o", "StrictHostKeyChecking=accept-new")
		}
		if server.Port != 0 {
			args = append(args, "-p", strconv.Itoa(server.Port))
		}
		if server.IdentityFile != "" {
			args = append(args, "-i", expandHome(server.IdentityFile))
		}
		target := server.Address
		if server.User != "" {
			target = server.User + "@" + target
		}
		args = append(args, target, "sh", "-s")
		cmd = exec.CommandContext(ctx, "ssh", args...)
	}
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	latency := time.Since(started)
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if ctx.Err() != nil {
			msg = "collection timed out"
		}
		return metrics.Sample{At: time.Now(), Online: false, Error: msg, Latency: latency}
	}
	s, err := Parse(stdout.String())
	s.Latency = latency
	if err != nil {
		s.Online = false
		s.Error = err.Error()
	} else {
		if server.IsLocal() && runtime.GOOS == "darwin" {
			if totals, idles, ok := darwinCPUTicks(); ok {
				s.CoreTotal, s.CoreIdle = totals, idles
				s.CPUTotal, s.CPUIdle = 0, 0
				for i := range totals {
					s.CPUTotal += totals[i]
					s.CPUIdle += idles[i]
				}
			}
		}
		if len(server.Disks) == 0 {
			return s
		}
		wanted := make(map[string]bool, len(server.Disks))
		for _, mount := range server.Disks {
			wanted[mount] = true
		}
		filtered := s.Disks[:0]
		for _, disk := range s.Disks {
			if wanted[disk.Mount] {
				filtered = append(filtered, disk)
			}
		}
		s.Disks = filtered
	}
	return s
}
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return h + p[1:]
		}
	}
	return p
}

const linuxScript = `
set -eu
[ -r /proc/stat ] || { echo 'servterm requires Linux' >&2; exit 1; }
printf 'hostname\t%s\n' "$(hostname 2>/dev/null || echo unknown)"
printf 'os\t%s\n' "$(. /etc/os-release 2>/dev/null && echo "${PRETTY_NAME:-Linux}" || echo Linux)"
printf 'kernel\t%s\n' "$(uname -r)"
awk '{print "uptime\t" $1}' /proc/uptime
awk '/^cpu /{$1=""; sub(/^ /,""); print "cpu\t" $0; exit}' /proc/stat
awk '/^cpu[0-9]+ /{$1=""; sub(/^ /,""); print "core\t" $0}' /proc/stat
awk '/^processor/{n++} END{print "cores\t" n+0}' /proc/cpuinfo
awk '{print "load\t" $1, $2, $3}' /proc/loadavg
awk '/^MemTotal:/{print "mem_total\t" $2} /^MemAvailable:/{print "mem_available\t" $2} /^SwapTotal:/{print "swap_total\t" $2} /^SwapFree:/{print "swap_free\t" $2}' /proc/meminfo
awk -F'[: ]+' 'NR>2 && $2!="lo" {rx+=$3; tx+=$11} END{print "net\t" rx+0, tx+0}' /proc/net/dev
for p in cpu memory io; do f="/proc/pressure/$p"; [ -r "$f" ] && awk -v p="$p" '/^some /{for(i=1;i<=NF;i++)if($i~/^avg10=/){split($i,a,"="); print "pressure_" p "\t" a[2]}}' "$f"; done
energy_uj=$(for f in /sys/class/powercap/intel-rapl*/energy_uj /sys/class/powercap/intel-rapl:*/*/energy_uj; do [ -r "$f" ] && cat "$f"; done | awk '{sum+=$1} END{if(NR) printf "%.0f",sum}')
[ -n "$energy_uj" ] && printf 'energy_uj\t%s\n' "$energy_uj"
power_uw=$(for f in /sys/class/power_supply/*/power_now; do [ -r "$f" ] && cat "$f"; done | awk '{sum+=$1} END{if(NR) printf "%.0f",sum}')
[ -n "$power_uw" ] && awk -v uw="$power_uw" 'BEGIN{printf "power_watts\t%.3f\n",uw/1000000}'
for b in /sys/class/power_supply/*; do
  [ -r "$b/capacity" ] || continue
  cap=$(cat "$b/capacity" 2>/dev/null); case "$cap" in ''|*[!0-9.]*) continue;; esac
  printf 'battery_percent\t%s\n' "$cap"
  status=$(cat "$b/status" 2>/dev/null || true); case "$status" in Charging|Full) printf 'battery_charging\ttrue\n';; esac
  break
done
df -PT -B1 2>/dev/null | awk 'NR>1 && $3 ~ /^[0-9]+$/ {print "disk\t" $1 "\t" $2 "\t" $3 "\t" $4 "\t" $7}'
for d in /sys/block/*; do [ -r "$d/size" ] || continue; n=${d##*/}; case "$n" in loop*|ram*) continue;; esac; sectors=$(cat "$d/size"); kind=ssd; [ -r "$d/queue/rotational" ] && [ "$(cat "$d/queue/rotational")" = 1 ] && kind=hdd; printf 'device\t%s %s %s\n' "$n" "$kind" "$((sectors*512))"; done
if [ -r /run/servterm/accelerators.tsv ]; then
  cat /run/servterm/accelerators.tsv
elif command -v nvidia-smi >/dev/null 2>&1; then
  nvidia-smi --query-gpu=name,utilization.gpu --format=csv,noheader,nounits 2>/dev/null | awk -F',' '{gsub(/^[ \t]+|[ \t]+$/,"",$1);gsub(/ /,"",$2);printf "accelerator\tGPU\t%s\ttrue\t%s\n",$1,$2}'
fi
if command -v lspci >/dev/null 2>&1; then
  lspci 2>/dev/null | awk -F': ' '
    /VGA compatible controller|3D controller|Display controller/ {name=$2;sub(/ \(rev .*/,"",name);if(name!~/NVIDIA/ && !system("test -r /run/servterm/accelerators.tsv"))next;if(name!~/NVIDIA/)printf "accelerator\tGPU\t%s\tfalse\t0\n",name}
    /GNA Scoring Accelerator|Neural.*Accelerator/ {name=$2;sub(/ \(rev .*/,"",name);printf "accelerator\tNPU\t%s\tfalse\t0\n",name}'
fi
ps -eo pid=,user=,comm=,pcpu=,pmem=,rss= --sort=-pcpu 2>/dev/null | awk 'NR<=12 {print "process\t" $1, $2, $3, $4, $5, $6}'
ps -eo pid=,ppid=,comm=,pcpu=,pmem=,rss= 2>/dev/null | awk '
{ids[NR]=$1; parent[$1]=$2; name[$1]=$3; cpu[$1]=$4; mem[$1]=$5; rss[$1]=$6}
END{
  for(i=1;i<=NR;i++){p=ids[i]; if(name[p]=="Runner.Listener")listeners++; if(name[p]=="Runner.Worker")jobs++}
  for(i=1;i<=NR;i++){id=ids[i]; p=id; owned=0; for(depth=0;depth<64&&p>1;depth++){if(name[p]=="Runner.Worker"||name[p]=="Runner.Listener"){owned=1;break};p=parent[p]};if(owned){treecpu+=cpu[id];treemem+=mem[id];treerss+=rss[id];treeprocs++}}
  printf "runners\t%d %d %.1f %.1f %.0f %d\n", listeners+0, jobs+0, treecpu+0, treemem+0, treerss+0, treeprocs+0
}'
runner_pids=$(ps -eo pid=,ppid=,comm= 2>/dev/null | awk '{ids[NR]=$1;parent[$1]=$2;name[$1]=$3} END{for(i=1;i<=NR;i++){id=ids[i];p=id;for(d=0;d<64&&p>1;d++){if(name[p]=="Runner.Worker"||name[p]=="Runner.Listener"){print id;break};p=parent[p]}}}')
ticks=0; for pid in $runner_pids; do [ -r "/proc/$pid/stat" ] || continue; values=$(cat "/proc/$pid/stat"); set -- $values; ticks=$((ticks+${14}+${15})); done; printf 'runner_ticks\t%s\n' "$ticks"
[ -r /run/servterm/runner-jobs.jsonl ] && while IFS= read -r job; do printf 'runner_job\t%s\n' "$job"; done < /run/servterm/runner-jobs.jsonl
`

const darwinScript = `
set -eu
[ "$(uname -s)" = Darwin ] || { echo 'servterm macOS sampler requires Darwin' >&2; exit 1; }
printf 'hostname\t%s\n' "$(hostname -s 2>/dev/null || hostname)"
printf 'os\tmacOS %s (%s)\n' "$(sw_vers -productVersion)" "$(sw_vers -buildVersion)"
printf 'kernel\t%s\n' "$(uname -r)"
boot=$(sysctl -n kern.boottime | awk -F'[=,]' '{gsub(/ /,"",$2); print $2}')
now=$(date +%s); awk -v n="$now" -v b="$boot" 'BEGIN{printf "uptime\t%.0f\n", n-b}'
top_output=$(top -l 1 -n 0 -stats cpu 2>/dev/null)
printf '%s\n' "$top_output" | awk '/^CPU usage:/ {gsub(/%/,"",$7); print "cpu_percent\t" 100-$7}'
cores=$(sysctl -n hw.logicalcpu); printf 'cores\t%s\n' "$cores"
sysctl -n vm.loadavg | tr -d '{},' | awk '{print "load\t" $1, $2, $3}'
mem_total=$(sysctl -n hw.memsize)
free_percent=$(memory_pressure 2>/dev/null | awk '/System-wide memory free percentage:/ {gsub(/%/,"",$5); print $5}')
if [ -n "$free_percent" ]; then
  mem_available=$(awk -v total="$mem_total" -v pct="$free_percent" 'BEGIN{printf "%.0f",total*pct/100}')
else
  mem_available=$(vm_stat | awk '
    /page size of/ {page=$8}
    /^Pages (free|inactive|speculative|purgeable):/ {v=$NF;gsub(/\./,"",v);pages+=v}
    END{printf "%.0f",pages*page}')
fi
printf 'mem_total_bytes\t%s\nmem_available_bytes\t%s\n' "$mem_total" "$mem_available"
sysctl -n vm.swapusage | awk '
function bytes(v, u){u=substr(v,length(v),1);v=substr(v,1,length(v)-1)+0;if(u=="K")return v*1024;if(u=="M")return v*1048576;if(u=="G")return v*1073741824;return v}
{for(i=1;i<=NF;i++){if($i=="total")t=bytes($(i+2));if($i=="free")f=bytes($(i+2))}} END{printf "swap_total_bytes\t%.0f\nswap_free_bytes\t%.0f\n",t,f}'
netstat -ibn | awk 'NR>1 && $1!="lo0" && $3~/^<Link#/ {rx+=$7;tx+=$10} END{print "net\t" rx+0,tx+0}'
pmset -g batt 2>/dev/null | awk '/InternalBattery|%/ {if (match($0,/[0-9]+%/)) {v=substr($0,RSTART,RLENGTH);gsub(/%/,"",v);print "battery_percent\t" v; if ($0 ~ /charging|charged/) print "battery_charging\ttrue"; exit}}'
smart_battery=$(ioreg -rn AppleSmartBattery -l 2>/dev/null || true)
printf '%s\n' "$smart_battery" | awk '
  /"Current" =/ {gsub(/[^0-9-]/,"",$0); current=$0}
  /"Voltage" =/ {gsub(/[^0-9]/,"",$0); voltage=$0}
  END {if (current!="" && voltage!="") {if (current<0) current=-current; printf "power_watts\t%.3f\n",current*voltage/1000000}}
'
accelerator_probe="$HOME/Library/Application Support/servterm/accelerators.tsv"
if [ -r "$accelerator_probe" ]; then cat "$accelerator_probe"; fi
gpu_info=$(ioreg -r -c IOAccelerator -l 2>/dev/null || true)
gpu_model=$(printf '%s\n' "$gpu_info" | awk -F'"' '/"model" =/{print $4;exit}')
gpu_util=$(printf '%s\n' "$gpu_info" | sed -n 's/.*"Device Utilization %"=\([0-9][0-9]*\).*/\1/p' | head -1)
if [ ! -r "$accelerator_probe" ] && [ -n "$gpu_model" ]; then
  if [ -n "$gpu_util" ]; then printf 'accelerator\tGPU\t%s\ttrue\t%s\n' "$gpu_model" "$gpu_util"; else printf 'accelerator\tGPU\t%s\tfalse\t0\n' "$gpu_model"; fi
fi
if [ ! -r "$accelerator_probe" ] && ioreg -r -c H11ANEIn -l 2>/dev/null | grep -q 'class H11ANEIn'; then
  printf 'accelerator\tNPU\tApple Neural Engine\tfalse\t0\n'
fi
df -Pk -l 2>/dev/null | awk 'NR>1 && $2~/^[0-9]+$/ {
  mount=$6
  if(mount=="/")next
  if(mount=="/System/Volumes/Data")mount="/"
  else if(mount~/^\/System\/Volumes\// || mount~/^\/Library\/Developer\//)next
  printf "disk\t%s\t%s\t%.0f\t%.0f\t%s\n",$1,$1,$2*1024,$3*1024,mount
}'
ps -axo pid=,user=,ucomm=,%cpu=,%mem=,rss= -r 2>/dev/null | awk 'NR<=12 {print "process\t" $1, $2, $3, $4, $5, $6}'
printf 'runners\t0 0 0 0 0 0\nrunner_ticks\t0\n'
`
