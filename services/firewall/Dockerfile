FROM python:3.11-slim

WORKDIR /app

RUN pip install --no-cache-dir fastapi uvicorn scikit-learn joblib pydantic pytest

COPY services/firewall/ services/firewall/

ENV PYTHONPATH=/app

# Pre-train model during image build
RUN python3 -m services.firewall.train

EXPOSE 5001

CMD ["python3", "-m", "uvicorn", "services.firewall.app:app", "--host", "0.0.0.0", "--port", "5001"]
