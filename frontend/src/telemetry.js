import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-web';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { SemanticResourceAttributes } from '@opentelemetry/semantic-conventions';
import { trace } from '@opentelemetry/api';

const exporter = new OTLPTraceExporter({
  url: '/v1/traces', // Relative to the current domain, Envoy will route it
});

const provider = new WebTracerProvider({
  resource: resourceFromAttributes({
    [SemanticResourceAttributes.SERVICE_NAME]: 'frontend',
  }),
  spanProcessors: [new BatchSpanProcessor(exporter)],
});

provider.register();

registerInstrumentations({
  instrumentations: [
    new DocumentLoadInstrumentation(),
    new FetchInstrumentation(),
    new XMLHttpRequestInstrumentation(),
    new UserInteractionInstrumentation(),
  ],
});

// Global Error Handling
const tracer = trace.getTracer('frontend-errors');

window.addEventListener('error', (event) => {
  const span = tracer.startSpan('unhandled-error');
  span.setStatus({ code: 2, message: event.message }); // 2 = Error
  span.setAttribute('error.type', event.error ? event.error.name : 'Error');
  span.setAttribute('error.message', event.message);
  if (event.error && event.error.stack) {
    span.setAttribute('error.stack', event.error.stack);
  }
  span.end();
});

window.addEventListener('unhandledrejection', (event) => {
  const span = tracer.startSpan('unhandled-rejection');
  span.setStatus({ code: 2, message: String(event.reason) });
  span.setAttribute('error.type', 'UnhandledRejection');
  span.setAttribute('error.message', String(event.reason));
  if (event.reason && event.reason.stack) {
    span.setAttribute('error.stack', event.reason.stack);
  }
  span.end();
});

console.log('OpenTelemetry initialized');
