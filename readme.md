# Kuda Finance AI

## Overview

Kuda Finance AI is a personal project that allows me track my Kuda account using AI.

> I am not in anyway affiliated with Kuda Bank. This is a completely personal project.

## Prerequisites

- Docker
- Docker Compose
- Git

## Getting Started

1. Clone the repository:

```bash
git clone https://github.com/lenajeremy/kuda-finance-public.git
cd kuda-finance-public
```

2. Copy the example environment file and configure your environment variables:

```bash
cp .env.example .env
# Edit .env with your configuration
```

3. Build and run using Docker Compose:

```bash
docker-compose up
```

This command will:

- Build all necessary containers
- Set up the required services
- Start the application

The application should now be running at `http://localhost:8080`

## Services

The following services are included in the docker-compose setup:

- API Server (coupled with client)
- Graph Database

## Development

To run in development mode:

```bash
docker-compose up --build
```

## Stopping the Application

To stop all services:

```bash
docker-compose down
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
