import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getTopAnalytics } from '../services/api';
import { ExternalLink, ChevronLeft, ChevronRight } from 'lucide-react';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "./ui/table";
import { Card } from "./ui/card";
import { Button } from "./ui/button";

export default function AnalyticsTable() {
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['analytics', page],
    queryFn: () => getTopAnalytics(page, pageSize),
    refetchInterval: 5000,
  });

  const analyticsData = data?.top_links || [];

  const handlePrevious = () => {
    if (page > 1) setPage(page - 1);
  };

  const handleNext = () => {
    if (data && data.top_links && data.top_links.length === pageSize) setPage(page + 1);
  };

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-12 space-y-4">
        <div className="w-12 h-12 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
        <p className="text-slate-500 animate-pulse">Loading analytics...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="py-12 text-center">
        <p className="text-red-500 font-medium">Failed to load analytics data.</p>
        <Button variant="link" onClick={() => refetch()} className="mt-2 text-blue-600">
          Try again
        </Button>
      </div>
    );
  }

  if (!analyticsData || analyticsData.length === 0) {
    return (
      <div className="py-12 text-center">
        <p className="text-slate-500">No analytics data available yet.</p>
      </div>
    );
  }

  return (
    <div className="w-full max-w-4xl">
      <Card className="overflow-hidden border-slate-200 shadow-sm">
        <Table>
          <TableHeader className="bg-slate-50/50">
            <TableRow>
              <TableHead className="px-6 py-4 font-bold text-slate-600">Short Link</TableHead>
              <TableHead className="px-6 py-4 font-bold text-slate-600">Destination</TableHead>
              <TableHead className="px-6 py-4 font-bold text-slate-600 text-right">Clicks</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {analyticsData.map((item) => (
              <TableRow key={item.short_code} className="hover:bg-slate-50/50 transition-colors group">
                <TableCell className="px-6 py-5">
                  <div className="flex flex-col">
                    <a
                      href={`${import.meta.env.VITE_SHORT_LINK_BASE_URL || 'http://localhost:10001'}/${item.short_code}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-bold text-blue-600 hover:underline"
                    >
                      {(import.meta.env.VITE_SHORT_LINK_BASE_URL || 'sho.rt').replace(/^https?:\/\//, '')}/{item.short_code}
                    </a>
                    <span className="text-[11px] text-slate-400 mt-1">
                      Created {item.created_at ? new Date(item.created_at).toLocaleDateString() : 'recently'}
                    </span>
                  </div>
                </TableCell>
                <TableCell className="px-6 py-5">
                  <div className="max-w-xs sm:max-w-md truncate text-slate-600 text-sm font-medium" title={item.long_url}>
                    {item.long_url}
                  </div>
                </TableCell>
                <TableCell className="px-6 py-5 text-right">
                  <div className="font-bold text-slate-900 text-lg">
                    {item.total_clicks.toLocaleString()}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>

      <div className="flex items-center justify-between mt-8">
        <p className="text-sm text-slate-500">
          Page <span className="font-semibold text-slate-900">{page}</span>
        </p>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="icon"
            onClick={handlePrevious}
            disabled={page === 1}
            className="border-slate-200"
          >
            <ChevronLeft className="w-5 h-5 text-slate-600" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            onClick={handleNext}
            disabled={analyticsData.length < pageSize}
            className="border-slate-200"
          >
            <ChevronRight className="w-5 h-5 text-slate-600" />
          </Button>
        </div>
      </div>
    </div>
  );
}
