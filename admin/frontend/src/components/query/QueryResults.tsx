import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Badge } from '../ui';
import { formatNumber, formatDuration } from '../../utils';
import type { QueryResult } from '../../types';

export interface QueryResultsProps {
  result: QueryResult;
  onExport?: (format: 'csv' | 'json') => void;
}

const QueryResults: React.FC<QueryResultsProps> = ({
  result,
  onExport
}) => {
  const [viewMode, setViewMode] = useState<'table' | 'json'>('table');
  const [currentPage, setCurrentPage] = useState(1);
  const rowsPerPage = 50;

  const totalPages = Math.ceil(result.rows.length / rowsPerPage);
  const startIndex = (currentPage - 1) * rowsPerPage;
  const endIndex = Math.min(startIndex + rowsPerPage, result.rows.length);
  const currentRows = result.rows.slice(startIndex, endIndex);

  const renderCellValue = (value: any): React.ReactNode => {
    if (value === null || value === undefined) {
      return <span className="text-gray-400 italic">NULL</span>;
    }

    if (typeof value === 'boolean') {
      return (
        <Badge variant={value ? 'success' : 'default'} size="sm">
          {value ? 'true' : 'false'}
        </Badge>
      );
    }

    if (typeof value === 'object') {
      return (
        <code className="text-xs bg-gray-100 px-2 py-1 rounded max-w-xs block truncate">
          {JSON.stringify(value)}
        </code>
      );
    }

    const stringValue = String(value);
    if (stringValue.length > 100) {
      return (
        <span className="block max-w-xs truncate" title={stringValue}>
          {stringValue}
        </span>
      );
    }

    return stringValue;
  };

  const renderTableView = () => (
    <div className="space-y-4">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider w-12">
                #
              </th>
              {result.columns.map((column, index) => (
                <th
                  key={index}
                  className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {currentRows.map((row, rowIndex) => (
              <tr key={startIndex + rowIndex} className="hover:bg-gray-50">
                <td className="px-4 py-3 text-sm text-gray-500 font-mono">
                  {startIndex + rowIndex + 1}
                </td>
                {row.map((cell, cellIndex) => (
                  <td
                    key={cellIndex}
                    className="px-4 py-3 text-sm text-gray-900"
                  >
                    {renderCellValue(cell)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between border-t border-gray-200 pt-4">
          <div className="text-sm text-gray-700">
            Showing {startIndex + 1} to {endIndex} of {formatNumber(result.rowCount)} results
          </div>
          <div className="flex items-center space-x-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
              disabled={currentPage <= 1}
            >
              Previous
            </Button>
            <span className="text-sm text-gray-700">
              Page {currentPage} of {totalPages}
            </span>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
              disabled={currentPage >= totalPages}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  );

  const renderJsonView = () => {
    const jsonData = result.rows.map(row => {
      const obj: Record<string, any> = {};
      result.columns.forEach((column, index) => {
        obj[column] = row[index];
      });
      return obj;
    });

    return (
      <div className="space-y-4">
        <pre className="bg-gray-50 p-4 rounded-lg overflow-x-auto text-sm">
          {JSON.stringify(jsonData.slice(startIndex, endIndex), null, 2)}
        </pre>
        
        {totalPages > 1 && (
          <div className="flex items-center justify-between border-t border-gray-200 pt-4">
            <div className="text-sm text-gray-700">
              Showing {startIndex + 1} to {endIndex} of {formatNumber(result.rowCount)} results
            </div>
            <div className="flex items-center space-x-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
                disabled={currentPage <= 1}
              >
                Previous
              </Button>
              <span className="text-sm text-gray-700">
                Page {currentPage} of {totalPages}
              </span>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
                disabled={currentPage >= totalPages}
              >
                Next
              </Button>
            </div>
          </div>
        )}
      </div>
    );
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Query Results</CardTitle>
            <div className="flex items-center space-x-4 mt-1 text-sm text-gray-600">
              <span>{formatNumber(result.rowCount)} rows</span>
              <span>{result.columns.length} columns</span>
              <span>Executed in {formatDuration(result.executionTime)}</span>
            </div>
          </div>
          <div className="flex items-center space-x-3">
            {/* View Mode Toggle */}
            <div className="flex items-center bg-gray-100 rounded-md p-1">
              <button
                onClick={() => setViewMode('table')}
                className={`px-3 py-1 text-sm rounded transition-colors ${
                  viewMode === 'table'
                    ? 'bg-white text-gray-900 shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                Table
              </button>
              <button
                onClick={() => setViewMode('json')}
                className={`px-3 py-1 text-sm rounded transition-colors ${
                  viewMode === 'json'
                    ? 'bg-white text-gray-900 shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                JSON
              </button>
            </div>

            {/* Export Options */}
            {onExport && (
              <div className="flex items-center space-x-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => onExport('csv')}
                >
                  Export CSV
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => onExport('json')}
                >
                  Export JSON
                </Button>
              </div>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {result.rows.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            <div className="text-gray-400 mb-4">
              <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">No Results</h3>
            <p className="text-gray-600">
              The query executed successfully but returned no rows.
            </p>
          </div>
        ) : (
          <>
            {viewMode === 'table' ? renderTableView() : renderJsonView()}
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default QueryResults;