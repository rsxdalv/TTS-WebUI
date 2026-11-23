import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { setNavigator } from "./navigation-bridge";

/**
 * Registers the navigate function of react-router in the bridge, so the
 * modules that navigate outside a component (the `next/router` default
 * export) use the same router as the components do.
 */
export const NavigatorRegistration = () => {
  const navigate = useNavigate();

  useEffect(() => {
    setNavigator((url, options) => navigate(url, options));
  }, [navigate]);

  return null;
};
