import { t } from "@lingui/core/macro"
import { Trans, useLingui } from "@lingui/react/macro"
import { useEffect, useMemo, useState } from "react"
import { CpuIcon, FingerprintIcon, MemoryStickIcon, TerminalSquareIcon, UserIcon, XIcon } from "lucide-react"
import { GpuIcon } from "@/components/ui/icons"
import { DataTablePagination, usePagination } from "@/components/ui/data-table-pagination"
import {
	type ColumnDef,
	type SortingState,
	flexRender,
	getCoreRowModel,
	getFilteredRowModel,
	getPaginationRowModel,
	getSortedRowModel,
	useReactTable,
} from "@tanstack/react-table"
import { pb } from "@/lib/api"
import { SystemStatus } from "@/lib/enums"
import { cn, decimalString } from "@/lib/utils"
import type { GPUProcess, GPUProcessSnapshot, SystemRecord, SystemStats } from "@/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

const columnIcons = {
	gpu: GpuIcon,
	pid: FingerprintIcon,
	user: UserIcon,
	name: TerminalSquareIcon,
	memory: MemoryStickIcon,
	usage: GpuIcon,
	rss: MemoryStickIcon,
	cpu: CpuIcon,
}

const emptyRows: GPUProcess[] = []
const percent = (value?: number) => (value === undefined ? "—" : `${decimalString(value, 1)}%`)
const memory = (value?: number) => {
	if (value === undefined) return "—"
	const unit = value >= 1024 ** 3 ? "GiB" : "MiB"
	return `${decimalString(value / (unit === "GiB" ? 1024 ** 3 : 1024 ** 2), 1)} ${unit}`
}

export default function GPUProcessesTable({ system }: { system: SystemRecord }) {
	const { i18n } = useLingui()
	const [snapshot, setSnapshot] = useState<GPUProcessSnapshot>()
	const [failed, setFailed] = useState(false)
	const [now, setNow] = useState(Date.now)
	const [filter, setFilter] = useState("")
	const { pagination, onPaginationChange, resetPageIndex } = usePagination()
	const [sorting, setSorting] = useState<SortingState>([{ id: "memory", desc: true }])
	useEffect(() => {
		setSnapshot(undefined)
		setFailed(false)
		if (system.status !== SystemStatus.Up) return
		let cancelled = false
		let unsubscribe = () => {}
		const timer = window.setInterval(() => setNow(Date.now()), 5000)
		pb.realtime
			.subscribe(
				"rt_metrics",
				(data: { stats: SystemStats }) => {
					if (cancelled) return
					setSnapshot(data.stats.gp)
					setNow(Date.now())
				},
				{ query: { system: system.id } }
			)
			.then((off) => {
				if (cancelled) off()
				else unsubscribe = off
			})
			.catch(() => {
				if (!cancelled) setFailed(true)
			})
		return () => {
			cancelled = true
			unsubscribe()
			window.clearInterval(timer)
		}
	}, [system.id, system.status])

	const columns = useMemo<ColumnDef<GPUProcess>[]>(
		() => [
			{ accessorKey: "gpu", header: "GPU" },
			{ accessorKey: "pid", header: "PID" },
			{ accessorKey: "user", header: () => t`User`, cell: ({ row }) => row.original.user || "—" },
			{ accessorKey: "name", header: () => t`Process` },
			{
				accessorKey: "memory",
				header: () => t`GPU Memory`,
				sortUndefined: "last",
				cell: ({ row }) => memory(row.original.memory),
			},
			{
				accessorKey: "usage",
				header: () => t`GPU Utilization`,
				sortUndefined: "last",
				cell: ({ row }) => percent(row.original.usage),
			},
			{
				accessorKey: "rss",
				header: () => t`Memory (RSS)`,
				sortUndefined: "last",
				cell: ({ row }) => memory(row.original.rss),
			},
			{ accessorKey: "cpu", header: "CPU", sortUndefined: "last", cell: ({ row }) => percent(row.original.cpu) },
		],
		[i18n.locale]
	)
	const stale = !!snapshot && now - snapshot.updated > 15000
	const available = system.status === SystemStatus.Up && snapshot?.status === "ok" && !stale && !failed
	const table = useReactTable({
		data: available ? (snapshot.processes ?? emptyRows) : emptyRows,
		columns,
		state: { sorting, globalFilter: filter, pagination },
		onSortingChange: (updater) => {
			setSorting(updater)
			resetPageIndex()
		},
		onGlobalFilterChange: (value) => {
			setFilter(value)
			resetPageIndex()
		},
		onPaginationChange,
		autoResetPageIndex: false,
		defaultColumn: { size: 100, minSize: 0 },
		getPaginationRowModel: getPaginationRowModel(),
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		getFilteredRowModel: getFilteredRowModel(),
		getColumnCanGlobalFilter: () => true,
		globalFilterFn: "includesString",
	})
	let message = t`Waiting for GPU process data. Update the agent if no data arrives.`
	if (system.status !== SystemStatus.Up)
		message = t`GPU process data is unavailable while the system is offline or paused.`
	else if (failed || snapshot?.status === "unavailable")
		message = t`Cannot read GPU processes. Check nvidia-smi pmon, driver support, and agent permissions.`
	else if (snapshot?.status === "unsupported") message = t`GPU process monitoring requires Linux and NVIDIA.`
	else if (stale) message = t`GPU process data is stale. Waiting for a new sample.`
	else if (available) message = filter ? t`No matching processes.` : t`No GPU processes are running.`
	return (
		<Card className="@container w-full px-3 py-5 sm:py-6 sm:px-6">
			<CardHeader className="p-0 mb-3 sm:mb-4">
				<div className="grid md:flex gap-x-5 gap-y-3 w-full items-end">
					<div className="px-2 sm:px-1">
						<CardTitle className="mb-2">
							<Trans>GPU Processes</Trans>
						</CardTitle>
						<CardDescription>
							<Trans>Live process usage. CPU: 100% per core.</Trans>
						</CardDescription>
					</div>
					<div className="relative ms-auto w-full max-w-full md:w-64">
						<Input
							aria-label={t`Filter GPU processes`}
							placeholder={t`Filter...`}
							value={filter}
							onChange={(e) => table.setGlobalFilter(e.target.value)}
							className="ps-4 pe-10 w-full"
						/>
						{filter && (
							<Button
								type="button"
								variant="ghost"
								size="icon"
								aria-label={t`Clear`}
								className="absolute end-1 top-1/2 -translate-y-1/2 size-7 text-muted-foreground"
								onClick={() => table.setGlobalFilter("")}
							>
								<XIcon />
							</Button>
						)}
					</div>
				</div>
			</CardHeader>
			<div className="max-w-full relative overflow-auto border rounded-md">
				<table className="text-sm w-full h-full text-nowrap">
					<TableHeader className="sticky top-0 w-full border-b-2">
						{table.getHeaderGroups().map((group) => (
							<tr key={group.id}>
								{group.headers.map((header) => {
									const isSorted = header.column.getIsSorted()
									const Icon = columnIcons[header.column.id as keyof typeof columnIcons]
									return (
										<TableHead
											key={header.id}
											className="px-2"
											style={{ width: header.getSize() }}
											aria-sort={isSorted === "asc" ? "ascending" : isSorted === "desc" ? "descending" : "none"}
										>
											<Button
												variant="ghost"
												className={cn(
													"h-9 px-3 flex items-center gap-2 duration-50",
													isSorted && "bg-accent/70 light:bg-accent text-accent-foreground/90"
												)}
												onClick={() => header.column.toggleSorting(isSorted === "asc")}
											>
												{Icon && <Icon data-icon="inline-start" />}
												{flexRender(header.column.columnDef.header, header.getContext())}
											</Button>
										</TableHead>
									)
								})}
							</tr>
						))}
					</TableHeader>
					<TableBody>
						{table.getRowModel().rows.map((row) => (
							<TableRow key={`${row.original.gpu}:${row.original.pid}`}>
								{row.getVisibleCells().map((cell) => (
									<TableCell
										key={cell.id}
										className="py-0 ps-4.5 tabular-nums"
										style={{ height: 54, width: cell.column.getSize() }}
									>
										{flexRender(cell.column.columnDef.cell, cell.getContext())}
									</TableCell>
								))}
							</TableRow>
						))}
						{!table.getRowModel().rows.length && (
							<TableRow>
								<TableCell colSpan={columns.length} className="h-37 text-center text-muted-foreground" role="status">
									{message}
								</TableCell>
							</TableRow>
						)}
					</TableBody>
				</table>
			</div>
			<DataTablePagination table={table} showSelected={false} />
		</Card>
	)
}
