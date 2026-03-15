import React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ExternalLink, Calendar, Link as LinkIcon, Pencil, Trash2, RefreshCw } from 'lucide-react';
import { updateUrl, deleteUrl, getUserHistory } from '../services/api';
import EditLinkModal from './EditLinkModal';
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

export default function MyLinks() {
  const queryClient = useQueryClient();
  const [editingLink, setEditingLink] = React.useState(null);
  const [isEditModalOpen, setIsEditModalOpen] = React.useState(false);

  const { data: history, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: ['user-history'],
    queryFn: getUserHistory,
  });

  const updateMutation = useMutation({
    mutationFn: ({ slug, longUrl }) => updateUrl(slug, longUrl),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-history'] });
      setIsEditModalOpen(false);
      setEditingLink(null);
    },
    onError: (err) => {
      console.error("Failed to update link:", err);
      alert("Failed to update link: " + (err.response?.data?.error || err.message));
    }
  });

  const deleteMutation = useMutation({
    mutationFn: (slug) => deleteUrl(slug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-history'] });
    },
    onError: (err) => {
      console.error("Failed to delete link:", err);
      alert("Failed to delete link: " + (err.response?.data?.error || err.message));
    }
  });

  const handleEdit = (link) => {
    setEditingLink(link);
    setIsEditModalOpen(true);
  };

  const handleDelete = (slug) => {
    if (window.confirm("Are you sure you want to delete this link?")) {
      deleteMutation.mutate(slug);
    }
  };

  const handleSaveEdit = (slug, longUrl) => {
    updateMutation.mutate({ slug, longUrl });
  };

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-12 space-y-4">
        <div className="w-12 h-12 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
        <p className="text-slate-500 animate-pulse">Loading your links...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="py-12 text-center">
        <p className="text-red-500 font-medium">Failed to load your links.</p>
        <Button variant="link" onClick={() => refetch()} className="mt-2 text-blue-600">
          Try again
        </Button>
      </div>
    );
  }

  return (
    <div className="w-full max-w-4xl animate-in fade-in slide-in-from-bottom-4 duration-500">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-slate-900 dark:text-white">My Links</h2>
        <Button 
          variant="outline" 
          size="sm" 
          onClick={() => refetch()} 
          disabled={isFetching}
          className="gap-2"
        >
          <RefreshCw className={`w-4 h-4 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {!history || history.length === 0 ? (
        <div className="py-12 text-center bg-slate-50 dark:bg-slate-900/50 rounded-2xl border-2 border-dashed border-slate-200 dark:border-slate-800">
          <div className="w-16 h-16 bg-slate-100 dark:bg-slate-800 rounded-full flex items-center justify-center mx-auto mb-4">
            <LinkIcon className="w-8 h-8 text-slate-400" />
          </div>
          <h3 className="text-lg font-bold text-slate-900 dark:text-white mb-2">No links yet</h3>
          <p className="text-slate-500 dark:text-slate-400 max-w-xs mx-auto">
            Start shortening URLs to see them here!
          </p>
        </div>
      ) : (
        <Card className="overflow-hidden border-slate-200 dark:border-slate-800 shadow-sm bg-white dark:bg-slate-950">
          <Table>
            <TableHeader className="bg-slate-50/50 dark:bg-slate-900/50">
              <TableRow>
                <TableHead className="px-6 py-4 font-bold text-slate-600 dark:text-slate-400">Short Link</TableHead>
                <TableHead className="px-6 py-4 font-bold text-slate-600 dark:text-slate-400">Destination</TableHead>
                <TableHead className="px-6 py-4 font-bold text-slate-600 dark:text-slate-400">Created</TableHead>
                <TableHead className="px-6 py-4 font-bold text-slate-600 dark:text-slate-400 text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {history.map((item) => (
                <TableRow key={item.id} className="hover:bg-slate-50/50 dark:hover:bg-slate-900/50 transition-colors group">
                  <TableCell className="px-6 py-5">
                    <div className="flex items-center gap-2">
                      <a
                        href={`${import.meta.env.VITE_SHORT_LINK_BASE_URL || 'http://localhost:10001'}/${item.id}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="font-bold text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
                      >
                        {(import.meta.env.VITE_SHORT_LINK_BASE_URL || 'sho.rt').replace(/^https?:\/\//, '')}/{item.id}
                        <ExternalLink className="w-3 h-3 opacity-0 group-hover:opacity-100 transition-opacity" />
                      </a>
                    </div>
                  </TableCell>
                  <TableCell className="px-6 py-5">
                    <div className="max-w-xs sm:max-w-md overflow-hidden text-slate-600 dark:text-slate-400 text-sm font-medium" title={item.long_url}>
                      {item.long_url}
                    </div>
                  </TableCell>
                  <TableCell className="px-6 py-5">
                    <div className="flex items-center gap-2 text-slate-500 dark:text-slate-500 text-sm">
                      <Calendar className="w-3 h-3" />
                      {new Date(item.created_at).toLocaleDateString(undefined, {
                        month: 'short',
                        day: 'numeric',
                        year: 'numeric'
                      })}
                    </div>
                  </TableCell>
                  <TableCell className="px-6 py-5 text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleEdit(item)}
                        className="h-8 w-8 text-slate-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
                        title="Edit link"
                      >
                        <Pencil className="w-4 h-4 pointer-events-none" />
                      </Button>
                      <button
                        onClick={(e) => {
                          e.preventDefault();
                          e.stopPropagation();
                          handleDelete(item.id);
                        }}
                        className="h-8 w-8 flex items-center justify-center text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-md transition-colors"
                        title="Delete link"
                      >
                        <Trash2 className="w-4 h-4 pointer-events-none" />
                      </button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      <EditLinkModal
        isOpen={isEditModalOpen}
        onClose={() => setIsEditModalOpen(false)}
        onSave={handleSaveEdit}
        link={editingLink}
        isSaving={updateMutation.isPending}
      />
    </div>
  );
}
