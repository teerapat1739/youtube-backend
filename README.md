# YouTube Activity Platform

A full-stack application for YouTube activity participation with Google OAuth authentication and subscription verification.

## 🏗️ Project Structure

```
youtube/
├── be/                     # Go Backend API
│   ├── cmd/api/           # Main API entry point
│   ├── pkg/               # Backend packages
│   └── ...
├── fe/                    # Vue.js Frontend
│   ├── src/               # Vue source code
│   ├── package.json       # Frontend dependencies
│   └── ...
└── README.md              # This file
```

## 🚀 Quick Start

### Prerequisites

- Go 1.23+ for backend
- Node.js 18+ for frontend
- Google OAuth credentials (for full functionality)

### 1. Start Backend (Port 8080)

```bash
cd be
go run cmd/api/main.go
```

### 2. Start Frontend (Port 3000)

```bash
cd fe
npm install
npm run dev
```

### 3. Access the Application

- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **Health Check**: http://localhost:8080/health

## 🔧 Configuration

### Backend Environment Variables

Create `be/.env` file:

```bash
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
JWT_SECRET=your-jwt-secret
PORT=8080
```

### Frontend Environment Variables

The frontend (`fe/.env`) is already configured:

```bash
VITE_API_URL=http://localhost:8080
```

## 📚 Documentation

- **Backend**: See `be/README.md` for detailed API documentation
- **Frontend**: See `fe/README.md` for frontend-specific information
- **Deployment**: Check `be/DEPLOYMENT.md` for production deployment

## 🎯 Features

- **Google OAuth Authentication**
- **YouTube Subscription Verification**
- **Activity Participation System**
- **Modern Vue.js Frontend**
- **RESTful Go API**
- **Cloud-Ready Deployment**

## 🔗 API Endpoints

### Authentication

- `GET /auth/google/login` - Google OAuth login
- `GET /auth/google/callback` - OAuth callback
- `POST /auth/be/login` - Backend authentication

### Activity

- `GET /api/check-subscription` - Check YouTube subscription
- `POST /api/join-activity` - Join activity

### User

- `GET /api/user-info` - Get user information
- `GET /api/youtube-subscriptions` - Get subscriptions

## 🛠️ Development

### Backend Development

```bash
cd be
go mod tidy
go run cmd/api/main.go
```

### Frontend Development

```bash
cd fe
npm install
npm run dev
```

### Git Workflow

```bash
git add .
git commit -m "Your commit message"
git push origin main
```

## 📝 License

This project is proprietary. All rights reserved.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📞 Support

For support and questions, please contact the development team.
