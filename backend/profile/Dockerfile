# Используем легковесный официальный образ Python
# так как стабильной версии 3.14 еще нет, используем актуальную 3.12 или 3.13)
FROM python:3.14-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

COPY requirements.txt .

RUN pip install --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000


CMD ["uvicorn", "cmd.profile.main:app", "--host", "0.0.0.0", "--port", "8000"]