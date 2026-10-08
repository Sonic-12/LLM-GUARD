import time
import statistics
import requests

PROXY_URL = "http://127.0.0.1:8080/v1/chat/completions"
PROMPT = "How does AES encryption work?"
ITERATIONS = 15


def main():
    body = {"messages": [{"role": "user", "content": PROMPT}]}
    timings = []

    for i in range(1, ITERATIONS + 1):
        start = time.perf_counter()
        resp = requests.post(PROXY_URL, json=body, timeout=90)
        elapsed_ms = (time.perf_counter() - start) * 1000

        request_id = resp.headers.get("X-LLMGuard-Request-Id", "N/A")
        timings.append({"run": i, "request_id": request_id, "total_ms": elapsed_ms})

    print(f"{'Run':<5} {'Request ID':<20} Total (ms)")
    print("-" * 45)
    for t in timings:
        print(f"{t['run']:<5} {t['request_id']:<20} {t['total_ms']:.1f}")

    values = [t["total_ms"] for t in timings]
    avg = statistics.mean(values)
    minimum = min(values)
    maximum = max(values)

    print(f"\n=== End-to-end totals across {ITERATIONS} runs ===")
    print(f"Average: {avg:.1f} ms")
    print(f"Min: {minimum:.1f} ms | Max: {maximum:.1f} ms")
    print("\nNext step: grep the proxy terminal log for these Request IDs and")
    print("read the 'pre_hooks=' value on each line - that isolates rules+DLP")
    print("cost from LLM generation time, matching Week 1's ~39.4ms methodology.")


if __name__ == "__main__":
    main()