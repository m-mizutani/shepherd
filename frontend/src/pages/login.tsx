import { useAuth } from "../contexts/auth-context";
import { Navigate } from "react-router-dom";
import { logoSrc } from "../components/ui/logo";
import { Skeleton } from "../components/ui/skeleton";
import { useTranslation } from "../i18n";

function SlackLogo({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 122 122" aria-hidden="true">
      <path fill="#E01E5A" d="M25.8,77.6c0,7.1-5.8,12.9-12.9,12.9S0,84.7,0,77.6s5.8-12.9,12.9-12.9h12.9V77.6z" />
      <path fill="#E01E5A" d="M32.3,77.6c0-7.1,5.8-12.9,12.9-12.9s12.9,5.8,12.9,12.9v32.3c0,7.1-5.8,12.9-12.9,12.9s-12.9-5.8-12.9-12.9V77.6z" />
      <path fill="#36C5F0" d="M45.2,25.8c-7.1,0-12.9-5.8-12.9-12.9S38.1,0,45.2,0s12.9,5.8,12.9,12.9v12.9H45.2z" />
      <path fill="#36C5F0" d="M45.2,32.3c7.1,0,12.9,5.8,12.9,12.9s-5.8,12.9-12.9,12.9H12.9C5.8,58.1,0,52.3,0,45.2s5.8-12.9,12.9-12.9H45.2z" />
      <path fill="#2EB67D" d="M97,45.2c0-7.1,5.8-12.9,12.9-12.9S122.7,38.1,122.7,45.2s-5.8,12.9-12.9,12.9H97V45.2z" />
      <path fill="#2EB67D" d="M90.5,45.2c0,7.1-5.8,12.9-12.9,12.9s-12.9-5.8-12.9-12.9V12.9C64.7,5.8,70.5,0,77.6,0s12.9,5.8,12.9,12.9V45.2z" />
      <path fill="#ECB22E" d="M77.6,97c7.1,0,12.9,5.8,12.9,12.9s-5.8,12.9-12.9,12.9s-12.9-5.8-12.9-12.9V97H77.6z" />
      <path fill="#ECB22E" d="M77.6,90.5c-7.1,0-12.9-5.8-12.9-12.9s5.8-12.9,12.9-12.9h32.3c7.1,0,12.9,5.8,12.9,12.9s-5.8,12.9-12.9,12.9H77.6z" />
    </svg>
  );
}

export default function LoginPage() {
  const { user, isLoading } = useAuth();
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <Skeleton width={140} height={14} />
      </div>
    );
  }

  if (user) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="min-h-screen flex flex-col bg-bg">
      <div
        className="flex-1 flex items-center justify-center px-4"
        style={{
          backgroundImage:
            "radial-gradient(circle at 20% 10%, #ffe4ce 0%, transparent 40%), radial-gradient(circle at 85% 90%, #fff1e6 0%, transparent 45%)",
        }}
      >
        <div className="w-[380px] max-w-full px-8 pt-9 pb-7 bg-bg-elev border border-line rounded-5 shadow-3 text-center">
          <div className="w-[72px] h-[72px] mx-auto mb-4 rounded-[18px] bg-brand-soft border border-[#f1d6b6] flex items-center justify-center">
            <img
              src={logoSrc}
              alt={t("appName")}
              className="w-16 h-16 object-contain"
            />
          </div>
          <h1 className="m-0 mb-1 text-[22px] font-semibold tracking-[-0.02em] text-ink-1">
            {t("appName")}
          </h1>
          <p className="m-0 mb-5 text-ink-3 text-[13.5px]">
            {t("loginSubtitle")}
          </p>
          <a
            href="/api/auth/login"
            className="w-full inline-flex items-center justify-center gap-2 h-[38px] px-4 rounded-2 bg-white text-ink-1 border border-ink-1 text-[14px] font-medium hover:bg-[#faf8f3] transition-colors"
          >
            <SlackLogo />
            {t("loginSlackButton")}
          </a>
          <div className="mt-4 text-[11.5px] text-ink-4">
            {t("loginPolicyNote")}
          </div>
        </div>
      </div>
      <div className="py-3.5 text-center text-[11.5px] text-ink-4">
        {t("loginFooterStatus")}{" "}
        <span className="text-success">● {t("appOperational")}</span>
      </div>
    </div>
  );
}
