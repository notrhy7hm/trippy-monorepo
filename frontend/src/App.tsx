import { Route, Routes } from "react-router-dom";
import { AppShell } from "./components/layout/AppShell";
import { RequireAuth } from "./components/RequireAuth";
import { Landing } from "./pages/Landing";
import { Login } from "./pages/Login";
import { Register } from "./pages/Register";
import { AppHome } from "./pages/AppHome";
import { Profile } from "./pages/Profile";
import { Trips } from "./pages/Trips";
import { NewTrip } from "./pages/NewTrip";
import { TripDashboard } from "./pages/TripDashboard";
import { NotFound } from "./pages/NotFound";

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      <Route element={<RequireAuth />}>
        <Route path="/app" element={<AppShell />}>
          <Route index element={<AppHome />} />
          <Route path="profile" element={<Profile />} />
          <Route path="trips" element={<Trips />} />
          <Route path="trips/new" element={<NewTrip />} />
          <Route path="trips/:tripSlug" element={<TripDashboard />} />
        </Route>
      </Route>

      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
