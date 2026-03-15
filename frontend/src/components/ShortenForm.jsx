import React, { useState } from 'react';
import { shortenUrl } from '../services/api';
import { useAuth } from '../context/AuthContext';
import { Copy, Check, ExternalLink, AlertCircle, Link2, Hash } from 'lucide-react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { cn } from '../lib/utils';

export default function ShortenForm() {
  const { user } = useAuth();
  const [longUrl, setLongUrl] = useState('');
  const [customSlug, setCustomSlug] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState(null);
  const [error, setError] = useState(null);
  const [copied, setCopied] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setResult(null);
    setCopied(false);

    try {
      const data = await shortenUrl(longUrl, user ? customSlug : undefined);
      setResult(data);
      setLongUrl('');
      setCustomSlug('');
    } catch (err) {
      setError(err.response?.data?.error || 'Something went wrong. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = () => {
    navigator.clipboard.writeText(result.short_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="w-full max-w-4xl">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex gap-4 items-start">
          <div className="relative flex-1">
            <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
              <Link2 className="h-5 w-5 text-slate-400" />
            </div>
            <Input
              type="url"
              required
              placeholder="https://very-long-url.com/some/path/to/resource"
              className="pl-11 h-14 text-lg rounded-xl border-slate-200 shadow-sm"
              value={longUrl}
              onChange={(e) => setLongUrl(e.target.value)}
            />
          </div>
          <Button
            type="submit"
            disabled={loading}
            size="lg"
            className="h-14 px-8 text-lg font-bold rounded-xl bg-blue-600 hover:bg-blue-700 shadow-md min-w-[180px]"
          >
            {loading ? 'Shortening...' : 'Shorten URL'}
          </Button>
        </div>

        {user && (
          <div className="flex gap-4 items-start animate-in fade-in slide-in-from-top-2 duration-300">
            <div className="relative flex-1 max-w-md">
              <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
                <Hash className="h-5 w-5 text-slate-400" />
              </div>
              <Input
                type="text"
                placeholder="Custom alias (optional)"
                className="pl-11 h-12 rounded-xl border-slate-200 shadow-sm"
                value={customSlug}
                onChange={(e) => setCustomSlug(e.target.value)}
              />
            </div>
          </div>
        )}
      </form>

      {error && (
        <div className="mt-6 p-4 bg-red-50 border border-red-200 rounded-lg flex items-start gap-3 text-red-700">
          <AlertCircle className="w-5 h-5 mt-0.5 flex-shrink-0" />
          <p className="text-sm">{error}</p>
        </div>
      )}

      {result && (
        <div className="mt-6 p-6 bg-blue-50 border border-blue-200 rounded-xl animate-in fade-in slide-in-from-top-4 duration-500">
          <p className="text-sm font-medium text-blue-900 mb-2">Success! Your short URL is ready:</p>
          <div className="flex items-center gap-2">
            <div className="flex-1 px-4 py-2 bg-white border border-blue-300 rounded-lg font-mono text-blue-600 truncate">
              {(import.meta.env.VITE_SHORT_LINK_BASE_URL || 'sho.rt').replace(/^https?:\/\//, '')}/{result.short_code || result.short_url.split('/').pop()}
            </div>
            <Button
              variant="outline"
              size="icon"
              onClick={copyToClipboard}
              className="bg-white border-blue-300 text-blue-600 hover:bg-blue-50"
            >
              {copied ? <Check className="w-5 h-5 text-green-500" /> : <Copy className="w-5 h-5" />}
            </Button>
            <Button
              variant="outline"
              size="icon"
              asChild
              className="bg-white border-blue-300 text-blue-600 hover:bg-blue-50"
            >
              <a href={result.short_url} target="_blank" rel="noopener noreferrer">
                <ExternalLink className="w-5 h-5" />
              </a>
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
