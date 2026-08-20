//go:build darwin && cgo

package collector

/*
#include <mach/mach.h>
#include <mach/processor_info.h>
#include <stdint.h>

static int servterm_cpu_ticks(uint64_t *totals, uint64_t *idles, int capacity) {
    natural_t cpu_count = 0;
    processor_info_array_t info = NULL;
    mach_msg_type_number_t info_count = 0;
    kern_return_t result = host_processor_info(
        mach_host_self(), PROCESSOR_CPU_LOAD_INFO, &cpu_count, &info, &info_count
    );
    if (result != KERN_SUCCESS || info == NULL || cpu_count == 0 || cpu_count > (natural_t)capacity) {
        if (info != NULL) {
            vm_deallocate(mach_task_self(), (vm_address_t)info, info_count * sizeof(integer_t));
        }
        return 0;
    }

    processor_cpu_load_info_t loads = (processor_cpu_load_info_t)info;
    for (natural_t cpu = 0; cpu < cpu_count; cpu++) {
        uint64_t user = loads[cpu].cpu_ticks[CPU_STATE_USER];
        uint64_t system = loads[cpu].cpu_ticks[CPU_STATE_SYSTEM];
        uint64_t nice = loads[cpu].cpu_ticks[CPU_STATE_NICE];
        uint64_t idle = loads[cpu].cpu_ticks[CPU_STATE_IDLE];
        totals[cpu] = user + system + nice + idle;
        idles[cpu] = idle;
    }
    vm_deallocate(mach_task_self(), (vm_address_t)info, info_count * sizeof(integer_t));
    return (int)cpu_count;
}
*/
import "C"

import "unsafe"

func darwinCPUTicks() ([]uint64, []uint64, bool) {
	const capacity = 256
	totals := make([]uint64, capacity)
	idles := make([]uint64, capacity)
	count := int(C.servterm_cpu_ticks(
		(*C.uint64_t)(unsafe.Pointer(&totals[0])),
		(*C.uint64_t)(unsafe.Pointer(&idles[0])),
		C.int(capacity),
	))
	if count < 1 || count > capacity {
		return nil, nil, false
	}
	return totals[:count], idles[:count], true
}
