package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type DiffViewer struct {
	viewport viewport.Model
	width    int
	height   int
}

func NewDiffViewer() DiffViewer {
	vp := viewport.New(0, 0)
	vp.SetContent(mockDiffContent())
	return DiffViewer{viewport: vp}
}

func (d DiffViewer) Update(msg tea.Msg) (DiffViewer, tea.Cmd) {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

func (d DiffViewer) View() string {
	return d.viewport.View()
}

func (d *DiffViewer) SetSize(w, h int) {
	d.width = w
	d.height = h
	d.viewport.Width = w
	d.viewport.Height = h
}

func mockDiffContent() string {
	return `diff --git a/src/middleware/jwt.ts b/src/middleware/jwt.ts
new file mode 100644
--- /dev/null
+++ b/src/middleware/jwt.ts
@@ -0,0 +1,28 @@
+import { Request, Response, NextFunction } from 'express';
+import { verifyToken } from '../utils/token';
+
+export function authMiddleware(req: Request, res: Response, next: NextFunction) {
+  const header = req.headers.authorization;
+  if (!header || !header.startsWith('Bearer ')) {
+    return res.status(401).json({ error: 'Missing token' });
+  }
+
+  const token = header.slice(7);
+  const payload = verifyToken(token);
+  if (!payload) {
+    return res.status(401).json({ error: 'Invalid token' });
+  }
+
+  req.user = payload;
+  next();
+}

diff --git a/src/routes/index.ts b/src/routes/index.ts
--- a/src/routes/index.ts
+++ b/src/routes/index.ts
@@ -1,6 +1,8 @@
 import { Router } from 'express';
+import authRouter from './auth';
 import healthRouter from './health';

 const router = Router();
+router.use('/auth', authRouter);
 router.use('/health', healthRouter);

diff --git a/src/routes/auth.ts b/src/routes/auth.ts
new file mode 100644
--- /dev/null
+++ b/src/routes/auth.ts
@@ -0,0 +1,32 @@
+import { Router, Request, Response } from 'express';
+import { generateToken, verifyToken } from '../utils/token';
+
+const router = Router();
+
+router.post('/login', async (req: Request, res: Response) => {
+  const { email, password } = req.body;
+  // TODO: validate against database
+  const token = generateToken({ email, role: 'user' });
+  res.json({ token, expiresIn: 3600 });
+});
+
+router.post('/refresh', async (req: Request, res: Response) => {
+  const { token } = req.body;
+  const payload = verifyToken(token);
+  if (!payload) {
+    return res.status(401).json({ error: 'Invalid token' });
+  }
+  const newToken = generateToken(payload);
+  res.json({ token: newToken, expiresIn: 3600 });
+});
+
+export default router;`
}
