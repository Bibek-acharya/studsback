# StudSphere API Documentation

## Base URL
```
http://localhost:8080/api/v1
```

## Authentication
All protected endpoints require a JWT token in the Authorization header:
```
Authorization: Bearer <token>
```

## Standard Response Format
```json
{
  "success": true,
  "data": {},
  "error": null,
  "meta": {}
}
```

---

## Public Endpoints

### Health Check
- `GET /health` - Server health status

### Authentication
- `POST /auth/register` - Register new user
- `POST /auth/login` - User login
- `POST /auth/send-otp` - Send OTP for verification
- `POST /auth/verify-otp` - Verify OTP
- `GET /auth/google` - Google OAuth login
- `GET /auth/google/callback` - Google OAuth callback

### Institution Authentication
- `POST /institutions/auth/register` - Register institution
- `POST /institutions/auth/login` - Institution login

### Scholarship Provider Authentication
- `POST /scholarship-providers/auth/register` - Register scholarship provider
- `POST /scholarship-providers/auth/login` - Scholarship provider login

### Colleges (Public)
- `GET /colleges` - List all colleges
- `GET /colleges/featured` - List featured colleges
- `GET /colleges/:id` - Get college by ID

### Universities (Public)
- `GET /universities` - List all universities
- `GET /universities/:id` - Get university by ID
- `GET /universities/:id/:tab` - Get university tab data

### Education Content (Public)
- `GET /education/rankings` - Get education rankings
- `GET /education/exams` - List exams
- `GET /education/exams/:id` - Get exam by ID
- `GET /education/scholarships` - List scholarships
- `GET /education/scholarships/:id` - Get scholarship by ID
- `GET /education/scholarships/:id/similar` - Get similar scholarships
- `POST /education/scholarships/:id/apply` - Apply for scholarship (Protected)
- `GET /education/courses` - List courses
- `GET /education/courses/:id` - Get course by ID
- `GET /education/courses/:id/details` - Get course details
- `GET /education/admissions` - List admissions
- `GET /education/news` - List news articles
- `GET /education/news/:id` - Get news article by ID
- `GET /education/events` - List events
- `GET /education/events/:id` - Get event by ID
- `GET /education/blogs` - List blogs (with pagination, search, category filter)
- `GET /education/blogs/:id` - Get blog post by ID

### AI Tools (Public)
- `POST /tools/scholarship-finder/recommendations` - Get scholarship recommendations
- `POST /tools/college-recommender/recommendations` - Get college recommendations

### Forum (Public + Protected)
- `GET /forum/posts` - List forum posts
- `GET /forum/posts/:id/comments` - Get post comments
- `GET /forum/communities` - List communities
- `POST /forum/posts` - Create post (Protected)
- `POST /forum/posts/:id/like` - Like post (Protected)
- `POST /forum/posts/:id/dislike` - Dislike post (Protected)
- `POST /forum/posts/:id/save` - Save post (Protected)
- `PUT /forum/posts/:id` - Update post (Protected)
- `DELETE /forum/posts/:id` - Delete post (Protected)
- `POST /forum/posts/:id/comments` - Add comment (Protected)
- `POST /forum/posts/:id/poll/vote` - Vote in poll (Protected)
- `POST /forum/upload` - Upload media (Protected)
- `POST /forum/communities/:id/join` - Join community (Protected)

### System (Public)
- `POST /system/contact` - Submit contact inquiry
- `GET /system/ads` - Get active ads (filtered by page)
- `GET /system/carousels` - Get carousel slides (filtered by page)

---

## Protected Endpoints (Student)

### Profile
- `GET /profile` - Get user profile
- `PUT /profile` - Update profile
- `POST /preferences` - Save onboarding preferences

### Counselling
- `POST /counselling/bookings` - Create counselling booking
- `GET /counselling/bookings/my` - Get my bookings

### Admissions
- `POST /admissions` - Create admission
- `GET /admissions/my` - Get my admissions
- `GET /admissions/:id` - Get admission by ID
- `PUT /admissions/:id` - Update admission
- `DELETE /admissions/:id` - Delete admission

### Scholarship Applications
- `GET /scholarships/my-applications` - Get my applications
- `GET /scholarships/applications/:id` - Get application by ID
- `PUT /scholarships/applications/:id` - Update application
- `DELETE /scholarships/applications/:id` - Delete application

### Messaging
- `GET /messages` - List messages
- `GET /messages/:id` - Get message by ID
- `POST /messages` - Send message
- `POST /messages/:id/reply` - Reply to message
- `GET /messages/contacts` - Get message contacts

### Calendar
- `GET /calendar/events` - List calendar events
- `GET /calendar/events/:id` - Get event by ID
- `POST /calendar/events` - Create event
- `PUT /calendar/events/:id` - Update event
- `DELETE /calendar/events/:id` - Delete event

### Sphere Invites
- `GET /invites` - List invites
- `GET /invites/:id` - Get invite by ID
- `PUT /invites/:id/accept` - Accept invite
- `PUT /invites/:id/decline` - Decline invite
- `PUT /invites/:id/save` - Save invite

### Bookmarks
- `GET /bookmarks` - List bookmarks
- `POST /bookmarks` - Create bookmark
- `DELETE /bookmarks/:id` - Delete bookmark
- `GET /bookmarks/:type` - Get bookmarks by type

### Notifications
- `GET /notifications` - List notifications
- `PUT /notifications/:id/read` - Mark notification as read
- `PUT /notifications/read-all` - Mark all as read

---

## Scholarship Provider Portal

### Dashboard & Analytics
- `GET /scholarship-providers/dashboard` - Get dashboard stats
- `GET /scholarship-providers/analytics` - Get analytics data

### Scholarship Management
- `POST /scholarship-providers/scholarships` - Create scholarship
- `GET /scholarship-providers/scholarships` - List scholarships
- `GET /scholarship-providers/scholarships/:id` - Get scholarship by ID
- `PUT /scholarship-providers/scholarships/:id` - Update scholarship
- `DELETE /scholarship-providers/scholarships/:id` - Delete scholarship

### Application Management
- `GET /scholarship-providers/applications` - List applications
- `GET /scholarship-providers/applications/:id` - Get application by ID
- `PUT /scholarship-providers/applications/:id/evaluate` - Evaluate application
- `PUT /scholarship-providers/applications/:id/status` - Update application status

### Interviews
- `GET /scholarship-providers/interviews` - List interviews
- `POST /scholarship-providers/interviews` - Schedule interview
- `PUT /scholarship-providers/interviews/:id` - Update interview

### Messaging
- `GET /scholarship-providers/messages` - List messages
- `POST /scholarship-providers/messages` - Send message
- `GET /scholarship-providers/messages/:id` - Get message by ID

### Profile & Settings
- `GET /scholarship-providers/profile` - Get profile
- `PUT /scholarship-providers/profile` - Update profile
- `GET /scholarship-providers/settings` - Get settings
- `PUT /scholarship-providers/settings` - Update settings

---

## Institution Dashboard

### Dashboard & Analytics
- `GET /institution/dashboard` - Get dashboard overview
- `GET /institution/analytics` - Get analytics data

### Program Management
- `GET /institution/programs` - List programs
- `GET /institution/programs/:id` - Get program by ID
- `POST /institution/programs` - Create program
- `PUT /institution/programs/:id` - Update program
- `DELETE /institution/programs/:id` - Delete program

### College Profile
- `GET /institution/profile` - Get profile
- `PUT /institution/profile` - Update profile
- `GET /institution/media` - List media
- `POST /institution/media` - Upload media
- `DELETE /institution/media/:id` - Delete media

### Counselling Management
- `GET /institution/counselling/sessions` - List sessions
- `GET /institution/counselling/bookings` - List bookings
- `PUT /institution/counselling/bookings/:id/status` - Update booking status

### Entrance Exam Management
- `GET /institution/entrances` - List entrances
- `GET /institution/entrances/:id` - Get entrance by ID
- `POST /institution/entrances` - Create entrance
- `PUT /institution/entrances/:id` - Update entrance
- `DELETE /institution/entrances/:id` - Delete entrance
- `GET /institution/entrances/:id/applicants` - Get entrance applicants

### Events Management
- `GET /institution/events` - List events
- `GET /institution/events/:id` - Get event by ID
- `POST /institution/events` - Create event
- `PUT /institution/events/:id` - Update event
- `DELETE /institution/events/:id` - Delete event

### News & Notices
- `GET /institution/news` - List news
- `GET /institution/news/:id` - Get news by ID
- `POST /institution/news` - Create news
- `PUT /institution/news/:id` - Update news
- `DELETE /institution/news/:id` - Delete news

### QMS (Quality Management)
- `GET /institution/qms` - List QMS records
- `GET /institution/qms/:id` - Get QMS record by ID
- `POST /institution/qms` - Create QMS record
- `PUT /institution/qms/:id` - Update QMS record
- `DELETE /institution/qms/:id` - Delete QMS record

### Messaging
- `GET /institution/messages` - List messages
- `GET /institution/messages/:id` - Get message by ID
- `POST /institution/messages` - Send message
- `GET /institution/messages/students` - Get student contacts

### Settings
- `GET /institution/settings` - Get settings
- `PUT /institution/settings` - Update settings
- `PUT /institution/settings/password` - Update password

### Scholarship Management
- `GET /institution/scholarships` - List scholarships
- `POST /institution/scholarships` - Create scholarship
- `PUT /institution/scholarships/:id` - Update scholarship
- `DELETE /institution/scholarships/:id` - Delete scholarship

### Admission Management
- `GET /institution/admissions` - List admissions
- `PUT /institution/admissions/:id/status` - Update admission status

### Scholarship Application Management
- `GET /institution/scholarship-applications` - List applications
- `PUT /institution/scholarship-applications/:id/status` - Update application status

---

## Admin Endpoints

### University Management
- `GET /admin/universities` - List universities
- `GET /admin/universities/:id` - Get university by ID
- `POST /admin/universities` - Create university
- `PUT /admin/universities/:id` - Update university
- `DELETE /admin/universities/:id` - Delete university

### College Management
- `GET /admin/colleges` - List colleges
- `GET /admin/colleges/:id` - Get college by ID
- `POST /admin/colleges` - Create college
- `PUT /admin/colleges/:id` - Update college
- `DELETE /admin/colleges/:id` - Delete college
- `PUT /admin/colleges/:id/approve` - Approve college
- `PUT /admin/colleges/:id/featured` - Toggle featured status

### Admission Management
- `GET /admin/admissions` - List all admissions
- `GET /admin/admissions/:id` - Get admission by ID
- `PUT /admin/admissions/:id/status` - Update admission status
- `GET /admin/admissions/college/:collegeId` - Get college admissions

### Scholarship Management
- `GET /admin/scholarships` - List all scholarships
- `GET /admin/scholarships/:id` - Get scholarship by ID
- `POST /admin/scholarships` - Create scholarship
- `PUT /admin/scholarships/:id` - Update scholarship
- `DELETE /admin/scholarships/:id` - Delete scholarship

### Scholarship Application Management
- `GET /admin/scholarship-applications` - List all applications
- `GET /admin/scholarship-applications/:id` - Get application by ID
- `PUT /admin/scholarship-applications/:id/status` - Update application status
- `GET /admin/scholarship-applications/scholarship/:scholarshipId` - Get by scholarship

### Contact Inquiries
- `GET /admin/inquiries` - List inquiries (with filters)
- `GET /admin/inquiries/:id` - Get inquiry by ID
- `PUT /admin/inquiries/:id/status` - Update inquiry status
- `DELETE /admin/inquiries/:id` - Delete inquiry

### Ad Management
- `GET /admin/ads` - List all ads (with filters)
- `GET /admin/ads/:id` - Get ad by ID
- `POST /admin/ads` - Create ad
- `PUT /admin/ads/:id` - Update ad
- `DELETE /admin/ads/:id` - Delete ad
- `POST /admin/ads/:id/click` - Track ad click

### Carousel Management
- `GET /admin/carousels` - List carousel slides
- `GET /admin/carousels/:id` - Get slide by ID
- `POST /admin/carousels` - Create slide
- `PUT /admin/carousels/:id` - Update slide
- `DELETE /admin/carousels/:id` - Delete slide
- `PUT /admin/carousels/reorder` - Reorder slides

### Users
- `GET /admin/users` - List users

---

## Total Endpoint Count

| Category | Count |
|----------|-------|
| Public (Auth + Content) | 35 |
| Protected (Student) | 36 |
| Scholarship Provider | 20 |
| Institution Dashboard | 40 |
| Admin | 28 |
| **Total** | **159** |
