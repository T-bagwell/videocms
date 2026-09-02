import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import Navbar from './components/Navbar.jsx';
import LoginPage from './pages/LoginPage.jsx';
import BrowsePage from './pages/BrowsePage.jsx';
import VideoDetailPage from './pages/VideoDetailPage.jsx';
import PlayerPage from './pages/PlayerPage.jsx';
import PlaylistsPage from './pages/PlaylistsPage.jsx';
import PlaylistDetailPage from './pages/PlaylistDetailPage.jsx';
import FavoritesPage from './pages/FavoritesPage.jsx';
import SeriesListPage from './pages/SeriesListPage.jsx';
import SeriesDetailPage from './pages/SeriesDetailPage.jsx';
import MusicPage from './pages/MusicPage.jsx';
import BooksPage from './pages/BooksPage.jsx';
import BookReaderPage from './pages/BookReaderPage.jsx';
import PhotosPage from './pages/PhotosPage.jsx';
import PublicPage from './pages/PublicPage.jsx';
import PublicVideoPage from './pages/PublicVideoPage.jsx';
import RequestsPage from './pages/RequestsPage.jsx';
import StatsPage from './pages/StatsPage.jsx';
import SubscriptionsPage from './pages/SubscriptionsPage.jsx';
import SettingsPage from './pages/SettingsPage.jsx';
import AdminPage from './pages/AdminPage.jsx';
import LivePage from './pages/LivePage.jsx';
import SharePage from './pages/SharePage.jsx';
import { useAuth } from './auth.jsx';
import { useTranslation } from 'react-i18next';

function RequireAuth({ children }) {
  const { user, loading } = useAuth();
  const location = useLocation();
  const { t } = useTranslation();
  if (loading) return <div className="page-loading">{t('common.loading')}</div>;
  if (!user) return <Navigate to="/login" state={{ from: location }} replace />;
  return children;
}

export default function App() {
  const { user } = useAuth();
  return (
    <div className="app">
      {user && <Navbar />}
      <main className="main">
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/public" element={<PublicPage />} />
          <Route path="/public/v/:id" element={<PublicVideoPage />} />
          <Route path="/share/:token" element={<SharePage />} />
          <Route
            path="/"
            element={
              <RequireAuth>
                <BrowsePage />
              </RequireAuth>
            }
          />
          <Route
            path="/video/:id"
            element={
              <RequireAuth>
                <VideoDetailPage />
              </RequireAuth>
            }
          />
          <Route
            path="/player/:id"
            element={
              <RequireAuth>
                <PlayerPage />
              </RequireAuth>
            }
          />
          <Route
            path="/playlists"
            element={
              <RequireAuth>
                <PlaylistsPage />
              </RequireAuth>
            }
          />
          <Route
            path="/playlists/:id"
            element={
              <RequireAuth>
                <PlaylistDetailPage />
              </RequireAuth>
            }
          />
          <Route
            path="/favorites"
            element={
              <RequireAuth>
                <FavoritesPage />
              </RequireAuth>
            }
          />
          <Route
            path="/series"
            element={
              <RequireAuth>
                <SeriesListPage />
              </RequireAuth>
            }
          />
          <Route
            path="/series/:id"
            element={
              <RequireAuth>
                <SeriesDetailPage />
              </RequireAuth>
            }
          />
          <Route
            path="/music"
            element={
              <RequireAuth>
                <MusicPage />
              </RequireAuth>
            }
          />
          <Route
            path="/music/:id"
            element={
              <RequireAuth>
                <MusicPage />
              </RequireAuth>
            }
          />
          <Route
            path="/books"
            element={
              <RequireAuth>
                <BooksPage />
              </RequireAuth>
            }
          />
          <Route
            path="/books/:id"
            element={
              <RequireAuth>
                <BookReaderPage />
              </RequireAuth>
            }
          />
          <Route
            path="/photos"
            element={
              <RequireAuth>
                <PhotosPage />
              </RequireAuth>
            }
          />
          <Route
            path="/photos/:albumId"
            element={
              <RequireAuth>
                <PhotosPage />
              </RequireAuth>
            }
          />
          <Route
            path="/requests"
            element={
              <RequireAuth>
                <RequestsPage />
              </RequireAuth>
            }
          />
          <Route
            path="/stats"
            element={
              <RequireAuth>
                <StatsPage />
              </RequireAuth>
            }
          />
          <Route
            path="/subscriptions"
            element={
              <RequireAuth>
                <SubscriptionsPage />
              </RequireAuth>
            }
          />
          <Route
            path="/settings"
            element={
              <RequireAuth>
                <SettingsPage />
              </RequireAuth>
            }
          />
          <Route
            path="/admin"
            element={
              <RequireAuth>
                <AdminPage />
              </RequireAuth>
            }
          />
          <Route
            path="/live/:id"
            element={
              <RequireAuth>
                <LivePage />
              </RequireAuth>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
}
