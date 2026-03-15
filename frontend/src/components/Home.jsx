import React from 'react';
import ShortenForm from './ShortenForm';
import AnalyticsTable from './AnalyticsTable';

export default function Home() {
  return (
    <div className="animate-in fade-in duration-500">
      <div className="mb-12">
        <h2 className="text-3xl font-bold text-slate-900 dark:text-white mb-8">
          Shorten your link
        </h2>
        <ShortenForm />
      </div>

      <div className="mt-20">
        <h2 className="text-2xl font-bold text-slate-900 dark:text-white mb-6">
          Top Performing Links
        </h2>
        <AnalyticsTable />
      </div>
    </div>
  );
}
