import { t } from "@lingui/core/macro"
import { Trans, useLingui } from "@lingui/react/macro"
import { useEffect, useMemo, useState } from "react"
import { ArrowUpDownIcon } from "lucide-react"
import {
	type ColumnDef,
	type SortingState,
	flexRender,
	getCoreRowModel,
	getFilteredRowModel,
	getSortedRowModel,
	useReactTable,
} from "@tanstack/react-table"
import { pb } from "@/lib/api"
import { SystemStatus } from "@/lib/enums"
import { decimalString } from "@/lib/utils"
import type { GPUProcess, GPUProcessSnapshot, SystemRecord, SystemStats } from "@/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

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
		state: { sorting, globalFilter: filter },
		onSortingChange: setSorting,
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
		<Card>
			<CardHeader>
				<CardTitle>
					<Trans>GPU Processes</Trans>
				</CardTitle>
				<CardDescription>
					<Trans>
						Live samples, independent of chart time range. One row per GPU and PID. CPU: 100% per logical core.
						Unavailable metrics: —.
					</Trans>
				</CardDescription>
				{snapshot && (
					<p className="text-xs text-muted-foreground">
						<Trans>Last sampled</Trans>: {new Date(snapshot.updated).toLocaleTimeString()}
					</p>
				)}
				<Input
					aria-label={t`Filter GPU processes`}
					placeholder={t`Filter GPU processes`}
					value={filter}
					onChange={(e) => setFilter(e.target.value)}
					className="max-w-sm"
				/>
			</CardHeader>
			<CardContent>
				<div className="max-h-96 overflow-auto">
					<Table>
						<TableHeader>
							{table.getHeaderGroups().map((group) => (
								<TableRow key={group.id}>
									{group.headers.map((header) => (
										<TableHead
											key={header.id}
											aria-sort={
												header.column.getIsSorted() === "asc"
													? "ascending"
													: header.column.getIsSorted() === "desc"
														? "descending"
														: "none"
											}
										>
											<Button variant="ghost" onClick={header.column.getToggleSortingHandler()}>
												{flexRender(header.column.columnDef.header, header.getContext())}
												<ArrowUpDownIcon data-icon="inline-end" />
											</Button>
										</TableHead>
									))}
								</TableRow>
							))}
						</TableHeader>
						<TableBody>
							{table.getRowModel().rows.map((row) => (
								<TableRow key={`${row.original.gpu}:${row.original.pid}`}>
									{row.getVisibleCells().map((cell) => (
										<TableCell key={cell.id} className="tabular-nums whitespace-nowrap">
											{flexRender(cell.column.columnDef.cell, cell.getContext())}
										</TableCell>
									))}
								</TableRow>
							))}
							{!table.getRowModel().rows.length && (
								<TableRow>
									<TableCell colSpan={columns.length} className="h-24 text-center text-muted-foreground" role="status">
										{message}
									</TableCell>
								</TableRow>
							)}
						</TableBody>
					</Table>
				</div>
			</CardContent>
		</Card>
	)
}
