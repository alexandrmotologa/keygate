import { useState, useEffect } from 'react';
import { UsageEvent } from '../types';

export function useEventStream(maxEvents = 50) {
  const [events, setEvents] = useState<UsageEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);

  useEffect(() => {
    let es: EventSource | null = null;
    let reconnectTimeout: number | undefined;

    const connect = () => {
      es = new EventSource('/v1/analytics.stream');

      es.onopen = () => {
        setIsConnected(true);
      };

      es.onmessage = (event) => {
        try {
          const parsed: UsageEvent = JSON.parse(event.data);
          setEvents((prev) => [parsed, ...prev.slice(0, maxEvents - 1)]);
        } catch {
          // ignore keep-alive or ping
        }
      };

      es.onerror = () => {
        setIsConnected(false);
        if (es) {
          es.close();
        }
        reconnectTimeout = window.setTimeout(connect, 3000);
      };
    };

    connect();

    return () => {
      if (es) es.close();
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
    };
  }, [maxEvents]);

  return { events, isConnected };
}
