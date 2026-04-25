import { Outlet } from "react-router-dom";
import Header from "@/components/Header";

function Root() {
  return (
    <div className="w-full min-h-full bg-zinc-100 dark:bg-zinc-800">
      <div className="w-full max-w-7xl mx-auto flex flex-row justify-start items-start">
        <Header />
        <main className="w-auto max-w-full flex-grow shrink flex flex-col justify-start items-start">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

export default Root;
